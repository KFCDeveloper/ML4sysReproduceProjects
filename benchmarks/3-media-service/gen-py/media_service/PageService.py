#
# Autogenerated by Thrift Compiler (0.12.0)
#
# DO NOT EDIT UNLESS YOU ARE SURE THAT YOU KNOW WHAT YOU ARE DOING
#
#  options string: py
#

from thrift.Thrift import TType, TMessageType, TFrozenDict, TException, TApplicationException
from thrift.protocol.TProtocol import TProtocolException
from thrift.TRecursive import fix_spec

import sys
import logging
from .ttypes import *
from thrift.Thrift import TProcessor
from thrift.transport import TTransport
all_structs = []


class Iface(object):
    def ReadPage(self, req_id, movie_id, review_start, review_stop, carrier):
        """
        Parameters:
         - req_id
         - movie_id
         - review_start
         - review_stop
         - carrier

        """
        pass


class Client(Iface):
    def __init__(self, iprot, oprot=None):
        self._iprot = self._oprot = iprot
        if oprot is not None:
            self._oprot = oprot
        self._seqid = 0

    def ReadPage(self, req_id, movie_id, review_start, review_stop, carrier):
        """
        Parameters:
         - req_id
         - movie_id
         - review_start
         - review_stop
         - carrier

        """
        self.send_ReadPage(req_id, movie_id, review_start, review_stop, carrier)
        return self.recv_ReadPage()

    def send_ReadPage(self, req_id, movie_id, review_start, review_stop, carrier):
        self._oprot.writeMessageBegin('ReadPage', TMessageType.CALL, self._seqid)
        args = ReadPage_args()
        args.req_id = req_id
        args.movie_id = movie_id
        args.review_start = review_start
        args.review_stop = review_stop
        args.carrier = carrier
        args.write(self._oprot)
        self._oprot.writeMessageEnd()
        self._oprot.trans.flush()

    def recv_ReadPage(self):
        iprot = self._iprot
        (fname, mtype, rseqid) = iprot.readMessageBegin()
        if mtype == TMessageType.EXCEPTION:
            x = TApplicationException()
            x.read(iprot)
            iprot.readMessageEnd()
            raise x
        result = ReadPage_result()
        result.read(iprot)
        iprot.readMessageEnd()
        if result.success is not None:
            return result.success
        if result.se is not None:
            raise result.se
        raise TApplicationException(TApplicationException.MISSING_RESULT, "ReadPage failed: unknown result")


class Processor(Iface, TProcessor):
    def __init__(self, handler):
        self._handler = handler
        self._processMap = {}
        self._processMap["ReadPage"] = Processor.process_ReadPage

    def process(self, iprot, oprot):
        (name, type, seqid) = iprot.readMessageBegin()
        if name not in self._processMap:
            iprot.skip(TType.STRUCT)
            iprot.readMessageEnd()
            x = TApplicationException(TApplicationException.UNKNOWN_METHOD, 'Unknown function %s' % (name))
            oprot.writeMessageBegin(name, TMessageType.EXCEPTION, seqid)
            x.write(oprot)
            oprot.writeMessageEnd()
            oprot.trans.flush()
            return
        else:
            self._processMap[name](self, seqid, iprot, oprot)
        return True

    def process_ReadPage(self, seqid, iprot, oprot):
        args = ReadPage_args()
        args.read(iprot)
        iprot.readMessageEnd()
        result = ReadPage_result()
        try:
            result.success = self._handler.ReadPage(args.req_id, args.movie_id, args.review_start, args.review_stop, args.carrier)
            msg_type = TMessageType.REPLY
        except TTransport.TTransportException:
            raise
        except ServiceException as se:
            msg_type = TMessageType.REPLY
            result.se = se
        except TApplicationException as ex:
            logging.exception('TApplication exception in handler')
            msg_type = TMessageType.EXCEPTION
            result = ex
        except Exception:
            logging.exception('Unexpected exception in handler')
            msg_type = TMessageType.EXCEPTION
            result = TApplicationException(TApplicationException.INTERNAL_ERROR, 'Internal error')
        oprot.writeMessageBegin("ReadPage", msg_type, seqid)
        result.write(oprot)
        oprot.writeMessageEnd()
        oprot.trans.flush()

# HELPER FUNCTIONS AND STRUCTURES


class ReadPage_args(object):
    """
    Attributes:
     - req_id
     - movie_id
     - review_start
     - review_stop
     - carrier

    """


    def __init__(self, req_id=None, movie_id=None, review_start=None, review_stop=None, carrier=None,):
        self.req_id = req_id
        self.movie_id = movie_id
        self.review_start = review_start
        self.review_stop = review_stop
        self.carrier = carrier

    def read(self, iprot):
        if iprot._fast_decode is not None and isinstance(iprot.trans, TTransport.CReadableTransport) and self.thrift_spec is not None:
            iprot._fast_decode(self, iprot, [self.__class__, self.thrift_spec])
            return
        iprot.readStructBegin()
        while True:
            (fname, ftype, fid) = iprot.readFieldBegin()
            if ftype == TType.STOP:
                break
            if fid == 1:
                if ftype == TType.I64:
                    self.req_id = iprot.readI64()
                else:
                    iprot.skip(ftype)
            elif fid == 2:
                if ftype == TType.STRING:
                    self.movie_id = iprot.readString().decode('utf-8') if sys.version_info[0] == 2 else iprot.readString()
                else:
                    iprot.skip(ftype)
            elif fid == 3:
                if ftype == TType.I32:
                    self.review_start = iprot.readI32()
                else:
                    iprot.skip(ftype)
            elif fid == 4:
                if ftype == TType.I32:
                    self.review_stop = iprot.readI32()
                else:
                    iprot.skip(ftype)
            elif fid == 5:
                if ftype == TType.MAP:
                    self.carrier = {}
                    (_ktype365, _vtype366, _size364) = iprot.readMapBegin()
                    for _i368 in range(_size364):
                        _key369 = iprot.readString().decode('utf-8') if sys.version_info[0] == 2 else iprot.readString()
                        _val370 = iprot.readString().decode('utf-8') if sys.version_info[0] == 2 else iprot.readString()
                        self.carrier[_key369] = _val370
                    iprot.readMapEnd()
                else:
                    iprot.skip(ftype)
            else:
                iprot.skip(ftype)
            iprot.readFieldEnd()
        iprot.readStructEnd()

    def write(self, oprot):
        if oprot._fast_encode is not None and self.thrift_spec is not None:
            oprot.trans.write(oprot._fast_encode(self, [self.__class__, self.thrift_spec]))
            return
        oprot.writeStructBegin('ReadPage_args')
        if self.req_id is not None:
            oprot.writeFieldBegin('req_id', TType.I64, 1)
            oprot.writeI64(self.req_id)
            oprot.writeFieldEnd()
        if self.movie_id is not None:
            oprot.writeFieldBegin('movie_id', TType.STRING, 2)
            oprot.writeString(self.movie_id.encode('utf-8') if sys.version_info[0] == 2 else self.movie_id)
            oprot.writeFieldEnd()
        if self.review_start is not None:
            oprot.writeFieldBegin('review_start', TType.I32, 3)
            oprot.writeI32(self.review_start)
            oprot.writeFieldEnd()
        if self.review_stop is not None:
            oprot.writeFieldBegin('review_stop', TType.I32, 4)
            oprot.writeI32(self.review_stop)
            oprot.writeFieldEnd()
        if self.carrier is not None:
            oprot.writeFieldBegin('carrier', TType.MAP, 5)
            oprot.writeMapBegin(TType.STRING, TType.STRING, len(self.carrier))
            for kiter371, viter372 in self.carrier.items():
                oprot.writeString(kiter371.encode('utf-8') if sys.version_info[0] == 2 else kiter371)
                oprot.writeString(viter372.encode('utf-8') if sys.version_info[0] == 2 else viter372)
            oprot.writeMapEnd()
            oprot.writeFieldEnd()
        oprot.writeFieldStop()
        oprot.writeStructEnd()

    def validate(self):
        return

    def __repr__(self):
        L = ['%s=%r' % (key, value)
             for key, value in self.__dict__.items()]
        return '%s(%s)' % (self.__class__.__name__, ', '.join(L))

    def __eq__(self, other):
        return isinstance(other, self.__class__) and self.__dict__ == other.__dict__

    def __ne__(self, other):
        return not (self == other)
all_structs.append(ReadPage_args)
ReadPage_args.thrift_spec = (
    None,  # 0
    (1, TType.I64, 'req_id', None, None, ),  # 1
    (2, TType.STRING, 'movie_id', 'UTF8', None, ),  # 2
    (3, TType.I32, 'review_start', None, None, ),  # 3
    (4, TType.I32, 'review_stop', None, None, ),  # 4
    (5, TType.MAP, 'carrier', (TType.STRING, 'UTF8', TType.STRING, 'UTF8', False), None, ),  # 5
)


class ReadPage_result(object):
    """
    Attributes:
     - success
     - se

    """


    def __init__(self, success=None, se=None,):
        self.success = success
        self.se = se

    def read(self, iprot):
        if iprot._fast_decode is not None and isinstance(iprot.trans, TTransport.CReadableTransport) and self.thrift_spec is not None:
            iprot._fast_decode(self, iprot, [self.__class__, self.thrift_spec])
            return
        iprot.readStructBegin()
        while True:
            (fname, ftype, fid) = iprot.readFieldBegin()
            if ftype == TType.STOP:
                break
            if fid == 0:
                if ftype == TType.STRUCT:
                    self.success = Page()
                    self.success.read(iprot)
                else:
                    iprot.skip(ftype)
            elif fid == 1:
                if ftype == TType.STRUCT:
                    self.se = ServiceException()
                    self.se.read(iprot)
                else:
                    iprot.skip(ftype)
            else:
                iprot.skip(ftype)
            iprot.readFieldEnd()
        iprot.readStructEnd()

    def write(self, oprot):
        if oprot._fast_encode is not None and self.thrift_spec is not None:
            oprot.trans.write(oprot._fast_encode(self, [self.__class__, self.thrift_spec]))
            return
        oprot.writeStructBegin('ReadPage_result')
        if self.success is not None:
            oprot.writeFieldBegin('success', TType.STRUCT, 0)
            self.success.write(oprot)
            oprot.writeFieldEnd()
        if self.se is not None:
            oprot.writeFieldBegin('se', TType.STRUCT, 1)
            self.se.write(oprot)
            oprot.writeFieldEnd()
        oprot.writeFieldStop()
        oprot.writeStructEnd()

    def validate(self):
        return

    def __repr__(self):
        L = ['%s=%r' % (key, value)
             for key, value in self.__dict__.items()]
        return '%s(%s)' % (self.__class__.__name__, ', '.join(L))

    def __eq__(self, other):
        return isinstance(other, self.__class__) and self.__dict__ == other.__dict__

    def __ne__(self, other):
        return not (self == other)
all_structs.append(ReadPage_result)
ReadPage_result.thrift_spec = (
    (0, TType.STRUCT, 'success', [Page, None], None, ),  # 0
    (1, TType.STRUCT, 'se', [ServiceException, None], None, ),  # 1
)
fix_spec(all_structs)
del all_structs

