/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package api

import (
	"fmt"
	"math"
	"math/big"

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	mpa_types "k8s.io/autoscaler/multidimensional-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1alpha1"
	vpa_types "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/autoscaler/vertical-pod-autoscaler/pkg/utils/limitrange"
	"k8s.io/klog/v2"

	vpa_api_util "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/utils/vpa"

	apiv1 "k8s.io/api/core/v1"
)

// NewCappingRecommendationProcessor constructs new RecommendationsProcessor that adjusts recommendation
// for given pod to obey MPA resources policy and container limits
func NewCappingRecommendationProcessor(limitsRangeCalculator limitrange.LimitRangeCalculator) RecommendationProcessor {
	return &cappingRecommendationProcessor{limitsRangeCalculator: limitsRangeCalculator}
}

type cappingRecommendationProcessor struct {
	limitsRangeCalculator limitrange.LimitRangeCalculator
}

type cappingAction string

var (
	cappedToMinAllowed             cappingAction = "capped to minAllowed"
	cappedToMaxAllowed             cappingAction = "capped to maxAllowed"
	cappedToLimit                  cappingAction = "capped to container limit"
	cappedProportionallyToMaxLimit cappingAction = "capped to fit Max in container LimitRange"
	cappedProportionallyToMinLimit cappingAction = "capped to fit Min in container LimitRange"
)

type roundingMode int

const (
	noRounding roundingMode = iota
	roundUpToFullUnit
	roundDownToFullUnit
)

// Apply returns a recommendation for the given pod, adjusted to obey policy and limits.
func (c *cappingRecommendationProcessor) Apply(
	podRecommendation *vpa_types.RecommendedPodResources,
	policy *vpa_types.PodResourcePolicy,
	conditions []mpa_types.MultidimPodAutoscalerCondition,
	pod *apiv1.Pod) (*vpa_types.RecommendedPodResources, ContainerToAnnotationsMap, error) {
	// TODO: Annotate if request enforced by maintaining proportion with limit and allowed limit range is in conflict with policy.

	if podRecommendation == nil && policy == nil {
		// If there is no recommendation and no policies have been defined then no recommendation can be computed.
		return nil, nil, nil
	}
	if podRecommendation == nil {
		// Policies have been specified. Create an empty recommendation so that the policies can be applied correctly.
		podRecommendation = new(vpa_types.RecommendedPodResources)
	}
	updatedRecommendations := []vpa_types.RecommendedContainerResources{}
	containerToAnnotationsMap := ContainerToAnnotationsMap{}
	limitAdjustedRecommendation, err := c.capProportionallyToPodLimitRange(podRecommendation.ContainerRecommendations, pod)
	if err != nil {
		return nil, nil, err
	}
	for _, containerRecommendation := range limitAdjustedRecommendation {
		container := getContainer(containerRecommendation.ContainerName, pod)

		if container == nil {
			klog.V(2).Infof("no matching Container found for recommendation %s", containerRecommendation.ContainerName)
			continue
		}

		containerLimitRange, err := c.limitsRangeCalculator.GetContainerLimitRangeItem(pod.Namespace)
		if err != nil {
			klog.Warningf("failed to fetch LimitRange for %v namespace", pod.Namespace)
		}
		updatedContainerResources, containerAnnotations, err := getCappedRecommendationForContainer(
			*container, &containerRecommendation, policy, containerLimitRange)

		if len(containerAnnotations) != 0 {
			containerToAnnotationsMap[containerRecommendation.ContainerName] = containerAnnotations
		}

		if err != nil {
			return nil, nil, fmt.Errorf("cannot update recommendation for container name %v", container.Name)
		}
		updatedRecommendations = append(updatedRecommendations, *updatedContainerResources)
	}
	return &vpa_types.RecommendedPodResources{ContainerRecommendations: updatedRecommendations}, containerToAnnotationsMap, nil
}

func applyPodLimitRange(resources []vpa_types.RecommendedContainerResources,
	pod *apiv1.Pod, limitRange apiv1.LimitRangeItem, resourceName apiv1.ResourceName,
	fieldGetter func(vpa_types.RecommendedContainerResources) *apiv1.ResourceList) []vpa_types.RecommendedContainerResources {
	minLimit := limitRange.Min[resourceName]
	maxLimit := limitRange.Max[resourceName]
	defaultLimit := limitRange.Default[resourceName]

	var sumLimit, sumRecommendation resource.Quantity
	for i, container := range pod.Spec.Containers {
		if i >= len(resources) {
			continue
		}
		limit := container.Resources.Limits[resourceName]
		request := container.Resources.Requests[resourceName]
		recommendation := (*fieldGetter(resources[i]))[resourceName]
		containerLimit, _ := getProportionalResourceLimit(resourceName, &limit, &request, &recommendation, &defaultLimit)
		if containerLimit != nil {
			sumLimit.Add(*containerLimit)
		}
		sumRecommendation.Add(recommendation)
	}
	if minLimit.Cmp(sumLimit) <= 0 && minLimit.Cmp(sumRecommendation) <= 0 && (maxLimit.IsZero() || maxLimit.Cmp(sumLimit) >= 0) {
		return resources
	}

	if minLimit.Cmp(sumRecommendation) > 0 && !sumLimit.IsZero() {
		for i := range pod.Spec.Containers {
			request := (*fieldGetter(resources[i]))[resourceName]
			var cappedContainerRequest *resource.Quantity
			if resourceName == apiv1.ResourceMemory {
				cappedContainerRequest, _ = scaleQuantityProportionallyMem(&request, &sumRecommendation, &minLimit, roundUpToFullUnit)
			} else {
				cappedContainerRequest, _ = scaleQuantityProportionallyCPU(&request, &sumRecommendation, &minLimit, noRounding)
			}
			(*fieldGetter(resources[i]))[resourceName] = *cappedContainerRequest
		}
		return resources
	}

	if sumLimit.IsZero() {
		return resources
	}

	var targetTotalLimit resource.Quantity
	if minLimit.Cmp(sumLimit) > 0 {
		targetTotalLimit = minLimit
	}
	if !maxLimit.IsZero() && maxLimit.Cmp(sumLimit) < 0 {
		targetTotalLimit = maxLimit
	}
	for i := range pod.Spec.Containers {
		limit := (*fieldGetter(resources[i]))[resourceName]
		var cappedContainerRequest *resource.Quantity
		if resourceName == apiv1.ResourceMemory {
			cappedContainerRequest, _ = scaleQuantityProportionallyMem(&limit, &sumLimit, &targetTotalLimit, roundDownToFullUnit)
		} else {
			cappedContainerRequest, _ = scaleQuantityProportionallyCPU(&limit, &sumLimit, &targetTotalLimit, noRounding)
		}
		(*fieldGetter(resources[i]))[resourceName] = *cappedContainerRequest
	}
	return resources
}


func (c *cappingRecommendationProcessor) capProportionallyToPodLimitRange(
	containerRecommendations []vpa_types.RecommendedContainerResources, pod *apiv1.Pod) ([]vpa_types.RecommendedContainerResources, error) {
	podLimitRange, err := c.limitsRangeCalculator.GetPodLimitRangeItem(pod.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error obtaining limit range: %s", err)
	}
	if podLimitRange == nil {
		return containerRecommendations, nil
	}
	getTarget := func(rl vpa_types.RecommendedContainerResources) *apiv1.ResourceList { return &rl.Target }
	getUpper := func(rl vpa_types.RecommendedContainerResources) *apiv1.ResourceList { return &rl.UpperBound }
	getLower := func(rl vpa_types.RecommendedContainerResources) *apiv1.ResourceList { return &rl.LowerBound }

	containerRecommendations = applyPodLimitRange(containerRecommendations, pod, *podLimitRange, apiv1.ResourceCPU, getUpper)
	containerRecommendations = applyPodLimitRange(containerRecommendations, pod, *podLimitRange, apiv1.ResourceMemory, getUpper)

	containerRecommendations = applyPodLimitRange(containerRecommendations, pod, *podLimitRange, apiv1.ResourceCPU, getTarget)
	containerRecommendations = applyPodLimitRange(containerRecommendations, pod, *podLimitRange, apiv1.ResourceMemory, getTarget)

	containerRecommendations = applyPodLimitRange(containerRecommendations, pod, *podLimitRange, apiv1.ResourceCPU, getLower)
	containerRecommendations = applyPodLimitRange(containerRecommendations, pod, *podLimitRange, apiv1.ResourceMemory, getLower)
	return containerRecommendations, nil
}

func getContainer(containerName string, pod *apiv1.Pod) *apiv1.Container {
	for i, container := range pod.Spec.Containers {
		if container.Name == containerName {
			return &pod.Spec.Containers[i]
		}
	}
	return nil
}

// getCappedRecommendationForContainer returns a recommendation for the given container, adjusted to obey policy and limits.
func getCappedRecommendationForContainer(
	container apiv1.Container,
	containerRecommendation *vpa_types.RecommendedContainerResources,
	policy *vpa_types.PodResourcePolicy, limitRange *apiv1.LimitRangeItem) (*vpa_types.RecommendedContainerResources, []string, error) {
	if containerRecommendation == nil {
		return nil, nil, fmt.Errorf("no recommendation available for container name %v", container.Name)
	}
	// containerPolicy can be nil (user does not have to configure it).
	containerPolicy := vpa_api_util.GetContainerResourcePolicy(container.Name, policy)
	containerControlledValues := vpa_api_util.GetContainerControlledValues(container.Name, policy)

	cappedRecommendations := containerRecommendation.DeepCopy()

	cappingAnnotations := make([]string, 0)

	process := func(recommendation apiv1.ResourceList, genAnnotations bool) {
		// TODO: Add anotation if limitRange is conflicting with VPA policy.
		limitAnnotations := applyContainerLimitRange(recommendation, container, limitRange)
		annotations := applyMPAPolicy(recommendation, containerPolicy)
		if genAnnotations {
			cappingAnnotations = append(cappingAnnotations, limitAnnotations...)
			cappingAnnotations = append(cappingAnnotations, annotations...)
		}
		// TODO: If limits and policy are conflicting, set some condition on the VPA.
		if containerControlledValues == vpa_types.ContainerControlledValuesRequestsOnly {
			annotations = capRecommendationToContainerLimit(recommendation, container)
			if genAnnotations {
				cappingAnnotations = append(cappingAnnotations, annotations...)
			}
		}
	}

	process(cappedRecommendations.Target, true)
	process(cappedRecommendations.LowerBound, false)
	process(cappedRecommendations.UpperBound, false)

	return cappedRecommendations, cappingAnnotations, nil
}

func getProportionalResourceLimit(resourceName core.ResourceName, originalLimit, originalRequest, recommendedRequest, defaultLimit *resource.Quantity) (*resource.Quantity, string) {
	if originalLimit == nil || originalLimit.Value() == 0 && defaultLimit != nil {
		originalLimit = defaultLimit
	}
	// originalLimit not set, don't set limit.
	if originalLimit == nil || originalLimit.Value() == 0 {
		return nil, ""
	}
	// recommendedRequest not set, don't set limit.
	if recommendedRequest == nil || recommendedRequest.Value() == 0 {
		return nil, ""
	}
	// originalLimit set but originalRequest not set - K8s will treat the pod as if they were equal,
	// recommend limit equal to request
	if originalRequest == nil || originalRequest.Value() == 0 {
		result := *recommendedRequest
		return &result, ""
	}
	// originalLimit and originalRequest are set. If they are equal recommend limit equal to request.
	if originalRequest.MilliValue() == originalLimit.MilliValue() {
		result := *recommendedRequest
		return &result, ""
	}
	if resourceName == core.ResourceCPU {
		result, capped := scaleQuantityProportionallyCPU( /*scaledQuantity=*/ originalLimit /*scaleBase=*/, originalRequest /*scaleResult=*/, recommendedRequest, noRounding)
		if !capped {
			return result, ""
		}
		return result, fmt.Sprintf(
			"%v: failed to keep limit to request ratio; capping limit to int64", resourceName)
	}
	result, capped := scaleQuantityProportionallyMem( /*scaledQuantity=*/ originalLimit /*scaleBase=*/, originalRequest /*scaleResult=*/, recommendedRequest, noRounding)
	if !capped {
		return result, ""
	}
	return result, fmt.Sprintf(
		"%v: failed to keep limit to request ratio; capping limit to int64", resourceName)
}

// scaleQuantityProportionallyCPU returns a value in milliunits which has the same proportion to scaledQuantity as scaleResult has to scaleBase.
// It also returns a bool indicating if it had to cap result to MaxInt64 milliunits.
func scaleQuantityProportionallyCPU(scaledQuantity, scaleBase, scaleResult *resource.Quantity, rounding roundingMode) (*resource.Quantity, bool) {
	originalMilli := big.NewInt(scaledQuantity.MilliValue())
	scaleBaseMilli := big.NewInt(scaleBase.MilliValue())
	scaleResultMilli := big.NewInt(scaleResult.MilliValue())
	var scaledOriginal big.Int
	scaledOriginal.Mul(originalMilli, scaleResultMilli)
	scaledOriginal.Div(&scaledOriginal, scaleBaseMilli)
	if scaledOriginal.IsInt64() {
		result := resource.NewMilliQuantity(scaledOriginal.Int64(), scaledQuantity.Format)
		if rounding == roundUpToFullUnit {
			result.RoundUp(resource.Scale(0))
		}
		if rounding == roundDownToFullUnit {
			result.Sub(*resource.NewMilliQuantity(999, result.Format))
			result.RoundUp(resource.Scale(0))
		}
		return result, false
	}
	return resource.NewMilliQuantity(math.MaxInt64, scaledQuantity.Format), true
}

// scaleQuantityProportionallyMem returns a value in whole units which has the same proportion to scaledQuantity as scaleResult has to scaleBase.
// It also returns a bool indicating if it had to cap result to MaxInt64 units.
func scaleQuantityProportionallyMem(scaledQuantity, scaleBase, scaleResult *resource.Quantity, rounding roundingMode) (*resource.Quantity, bool) {
	originalValue := big.NewInt(scaledQuantity.Value())
	scaleBaseValue := big.NewInt(scaleBase.Value())
	scaleResultValue := big.NewInt(scaleResult.Value())
	var scaledOriginal big.Int
	scaledOriginal.Mul(originalValue, scaleResultValue)
	scaledOriginal.Div(&scaledOriginal, scaleBaseValue)
	if scaledOriginal.IsInt64() {
		result := resource.NewQuantity(scaledOriginal.Int64(), scaledQuantity.Format)
		if rounding == roundUpToFullUnit {
			result.RoundUp(resource.Scale(0))
		}
		if rounding == roundDownToFullUnit {
			result.Sub(*resource.NewMilliQuantity(999, result.Format))
			result.RoundUp(resource.Scale(0))
		}
		return result, false
	}
	return resource.NewQuantity(math.MaxInt64, scaledQuantity.Format), true
}

// applyContainerLimitRange updates recommendation if recommended resources are outside of limits defined in VPA resources policy
func applyContainerLimitRange(recommendation apiv1.ResourceList, container apiv1.Container, limitRange *apiv1.LimitRangeItem) []string {
	annotations := make([]string, 0)
	if limitRange == nil {
		return annotations
	}
	maxAllowedRecommendation := getMaxAllowedRecommendation(recommendation, container, limitRange)
	minAllowedRecommendation := getMinAllowedRecommendation(recommendation, container, limitRange)
	for resourceName, recommended := range recommendation {
		cappedToMin, isCapped := maybeCapToMin(recommended, resourceName, minAllowedRecommendation)
		recommendation[resourceName] = cappedToMin
		if isCapped {
			annotations = append(annotations, toCappingAnnotation(resourceName, cappedProportionallyToMinLimit))
		}
		cappedToMax, isCapped := maybeCapToMax(cappedToMin, resourceName, maxAllowedRecommendation)
		recommendation[resourceName] = cappedToMax
		if isCapped {
			annotations = append(annotations, toCappingAnnotation(resourceName, cappedProportionallyToMaxLimit))
		}
	}
	return annotations
}

func getMaxAllowedRecommendation(recommendation apiv1.ResourceList, container apiv1.Container,
	podLimitRange *apiv1.LimitRangeItem) apiv1.ResourceList {
	if podLimitRange == nil {
		return apiv1.ResourceList{}
	}
	return getBoundaryRecommendation(recommendation, container, podLimitRange.Max, podLimitRange.Default)
}

func getMinAllowedRecommendation(recommendation apiv1.ResourceList, container apiv1.Container,
	podLimitRange *apiv1.LimitRangeItem) apiv1.ResourceList {
	// Both limit and request must be higher than min set in the limit range:
	// https://github.com/kubernetes/kubernetes/blob/016e9d5c06089774c6286fd825302cbae661a446/plugin/pkg/admission/limitranger/admission.go#L303
	if podLimitRange == nil {
		return apiv1.ResourceList{}
	}
	minForLimit := getBoundaryRecommendation(recommendation, container, podLimitRange.Min, podLimitRange.Default)
	minForRequest := podLimitRange.Min
	if minForRequest == nil {
		return minForLimit
	}
	result := minForLimit
	if minForRequest.Cpu() != nil && minForRequest.Cpu().Cmp(*minForLimit.Cpu()) > 0 {
		result[apiv1.ResourceCPU] = *minForRequest.Cpu()
	}
	if minForRequest.Memory() != nil && minForRequest.Memory().Cmp(*minForLimit.Memory()) > 0 {
		result[apiv1.ResourceMemory] = *minForRequest.Memory()
	}
	return result
}

func getBoundaryRecommendation(recommendation apiv1.ResourceList, container apiv1.Container,
	boundaryLimit, defaultLimit apiv1.ResourceList) apiv1.ResourceList {
	if boundaryLimit == nil {
		return apiv1.ResourceList{}
	}
	boundaryCpu := vpa_api_util.GetBoundaryRequest(apiv1.ResourceCPU, container.Resources.Requests.Cpu(), container.Resources.Limits.Cpu(), boundaryLimit.Cpu(), defaultLimit.Cpu())
	boundaryMem := vpa_api_util.GetBoundaryRequest(apiv1.ResourceMemory, container.Resources.Requests.Memory(), container.Resources.Limits.Memory(), boundaryLimit.Memory(), defaultLimit.Memory())
	return apiv1.ResourceList{
		apiv1.ResourceCPU:    *boundaryCpu,
		apiv1.ResourceMemory: *boundaryMem,
	}
}

func maybeCapToPolicyMin(recommended resource.Quantity, resourceName apiv1.ResourceName,
	containerPolicy *vpa_types.ContainerResourcePolicy) (resource.Quantity, bool) {
	return maybeCapToMin(recommended, resourceName, containerPolicy.MinAllowed)
}

func maybeCapToPolicyMax(recommended resource.Quantity, resourceName apiv1.ResourceName,
	containerPolicy *vpa_types.ContainerResourcePolicy) (resource.Quantity, bool) {
	return maybeCapToMax(recommended, resourceName, containerPolicy.MaxAllowed)
}

func maybeCapToMin(recommended resource.Quantity, resourceName apiv1.ResourceName,
	min apiv1.ResourceList) (resource.Quantity, bool) {
	minResource, found := min[resourceName]
	if found && !minResource.IsZero() && recommended.Cmp(minResource) < 0 {
		return minResource, true
	}
	return recommended, false
}

func maybeCapToMax(recommended resource.Quantity, resourceName apiv1.ResourceName,
	max apiv1.ResourceList) (resource.Quantity, bool) {
	maxResource, found := max[resourceName]
	if found && !maxResource.IsZero() && recommended.Cmp(maxResource) > 0 {
		return maxResource, true
	}
	return recommended, false
}

func toCappingAnnotation(resourceName apiv1.ResourceName, action cappingAction) string {
	return fmt.Sprintf("%s %s", resourceName, action)
}

// applyMPAPolicy updates recommendation if recommended resources are outside of limits defined in VPA resources policy
func applyMPAPolicy(recommendation apiv1.ResourceList, policy *vpa_types.ContainerResourcePolicy) []string {
	if policy == nil {
		return nil
	}
	annotations := make([]string, 0)
	for resourceName, recommended := range recommendation {
		cappedToMin, isCapped := maybeCapToPolicyMin(recommended, resourceName, policy)
		recommendation[resourceName] = cappedToMin
		if isCapped {
			annotations = append(annotations, toCappingAnnotation(resourceName, cappedToMinAllowed))
		}
		cappedToMax, isCapped := maybeCapToPolicyMax(cappedToMin, resourceName, policy)
		recommendation[resourceName] = cappedToMax
		if isCapped {
			annotations = append(annotations, toCappingAnnotation(resourceName, cappedToMaxAllowed))
		}
	}
	return annotations
}

// capRecommendationToContainerLimit makes sure recommendation is not above current limit for the container.
// If this function makes adjustments appropriate annotations are returned.
func capRecommendationToContainerLimit(recommendation apiv1.ResourceList, container apiv1.Container) []string {
	annotations := make([]string, 0)
	// Iterate over limits set in the container. Unset means Infinite limit.
	for resourceName, limit := range container.Resources.Limits {
		recommendedValue, found := recommendation[resourceName]
		if found && recommendedValue.MilliValue() > limit.MilliValue() {
			recommendation[resourceName] = limit
			annotations = append(annotations, toCappingAnnotation(resourceName, cappedToLimit))
		}
	}
	return annotations
}
