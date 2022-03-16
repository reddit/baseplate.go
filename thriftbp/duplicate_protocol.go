package thriftbp

import (
	"context"

	"github.com/apache/thrift/lib/go/thrift"
)

// TODO: thrift 0.17.0 will provide TDuplicateToProtocol and we can remove this
// one when that's released.
type tDuplicateToProtocol struct {
	// Required. The actual TProtocol to do the read/write.
	Delegate thrift.TProtocol

	// Required. An TProtocol to duplicate everything read/written from Delegate.
	//
	// A typical use case of this is to use TSimpleJSONProtocol wrapping
	// TMemoryBuffer in a middleware to json logging requests/responses,
	// or wrapping a TTransport that counts bytes written to get the payload
	// sizes.
	//
	// DuplicateTo will be used as write only. For read calls on
	// TDuplicateToProtocol, the result read from Delegate will be written
	// to DuplicateTo.
	DuplicateTo thrift.TProtocol
}

func (tdtp *tDuplicateToProtocol) WriteMessageBegin(ctx context.Context, name string, typeID thrift.TMessageType, seqID int32) error {
	err := tdtp.Delegate.WriteMessageBegin(ctx, name, typeID, seqID)
	tdtp.DuplicateTo.WriteMessageBegin(ctx, name, typeID, seqID)
	return err
}

func (tdtp *tDuplicateToProtocol) WriteMessageEnd(ctx context.Context) error {
	err := tdtp.Delegate.WriteMessageEnd(ctx)
	tdtp.DuplicateTo.WriteMessageEnd(ctx)
	return err
}

func (tdtp *tDuplicateToProtocol) WriteStructBegin(ctx context.Context, name string) error {
	err := tdtp.Delegate.WriteStructBegin(ctx, name)
	tdtp.DuplicateTo.WriteStructBegin(ctx, name)
	return err
}

func (tdtp *tDuplicateToProtocol) WriteStructEnd(ctx context.Context) error {
	err := tdtp.Delegate.WriteStructEnd(ctx)
	tdtp.DuplicateTo.WriteStructEnd(ctx)
	return err
}

func (tdtp *tDuplicateToProtocol) WriteFieldBegin(ctx context.Context, name string, typeID thrift.TType, id int16) error {
	err := tdtp.Delegate.WriteFieldBegin(ctx, name, typeID, id)
	tdtp.DuplicateTo.WriteFieldBegin(ctx, name, typeID, id)
	return err
}

func (tdtp *tDuplicateToProtocol) WriteFieldEnd(ctx context.Context) error {
	err := tdtp.Delegate.WriteFieldEnd(ctx)
	tdtp.DuplicateTo.WriteFieldEnd(ctx)
	return err
}

func (tdtp *tDuplicateToProtocol) WriteFieldStop(ctx context.Context) error {
	err := tdtp.Delegate.WriteFieldStop(ctx)
	tdtp.DuplicateTo.WriteFieldStop(ctx)
	return err
}

func (tdtp *tDuplicateToProtocol) WriteMapBegin(ctx context.Context, keyType thrift.TType, valueType thrift.TType, size int) error {
	err := tdtp.Delegate.WriteMapBegin(ctx, keyType, valueType, size)
	tdtp.DuplicateTo.WriteMapBegin(ctx, keyType, valueType, size)
	return err
}

func (tdtp *tDuplicateToProtocol) WriteMapEnd(ctx context.Context) error {
	err := tdtp.Delegate.WriteMapEnd(ctx)
	tdtp.DuplicateTo.WriteMapEnd(ctx)
	return err
}

func (tdtp *tDuplicateToProtocol) WriteListBegin(ctx context.Context, elemType thrift.TType, size int) error {
	err := tdtp.Delegate.WriteListBegin(ctx, elemType, size)
	tdtp.DuplicateTo.WriteListBegin(ctx, elemType, size)
	return err
}

func (tdtp *tDuplicateToProtocol) WriteListEnd(ctx context.Context) error {
	err := tdtp.Delegate.WriteListEnd(ctx)
	tdtp.DuplicateTo.WriteListEnd(ctx)
	return err
}

func (tdtp *tDuplicateToProtocol) WriteSetBegin(ctx context.Context, elemType thrift.TType, size int) error {
	err := tdtp.Delegate.WriteSetBegin(ctx, elemType, size)
	tdtp.DuplicateTo.WriteSetBegin(ctx, elemType, size)
	return err
}

func (tdtp *tDuplicateToProtocol) WriteSetEnd(ctx context.Context) error {
	err := tdtp.Delegate.WriteSetEnd(ctx)
	tdtp.DuplicateTo.WriteSetEnd(ctx)
	return err
}

func (tdtp *tDuplicateToProtocol) WriteBool(ctx context.Context, value bool) error {
	err := tdtp.Delegate.WriteBool(ctx, value)
	tdtp.DuplicateTo.WriteBool(ctx, value)
	return err
}

func (tdtp *tDuplicateToProtocol) WriteByte(ctx context.Context, value int8) error {
	err := tdtp.Delegate.WriteByte(ctx, value)
	tdtp.DuplicateTo.WriteByte(ctx, value)
	return err
}

func (tdtp *tDuplicateToProtocol) WriteI16(ctx context.Context, value int16) error {
	err := tdtp.Delegate.WriteI16(ctx, value)
	tdtp.DuplicateTo.WriteI16(ctx, value)
	return err
}

func (tdtp *tDuplicateToProtocol) WriteI32(ctx context.Context, value int32) error {
	err := tdtp.Delegate.WriteI32(ctx, value)
	tdtp.DuplicateTo.WriteI32(ctx, value)
	return err
}

func (tdtp *tDuplicateToProtocol) WriteI64(ctx context.Context, value int64) error {
	err := tdtp.Delegate.WriteI64(ctx, value)
	tdtp.DuplicateTo.WriteI64(ctx, value)
	return err
}

func (tdtp *tDuplicateToProtocol) WriteDouble(ctx context.Context, value float64) error {
	err := tdtp.Delegate.WriteDouble(ctx, value)
	tdtp.DuplicateTo.WriteDouble(ctx, value)
	return err
}

func (tdtp *tDuplicateToProtocol) WriteString(ctx context.Context, value string) error {
	err := tdtp.Delegate.WriteString(ctx, value)
	tdtp.DuplicateTo.WriteString(ctx, value)
	return err
}

func (tdtp *tDuplicateToProtocol) WriteBinary(ctx context.Context, value []byte) error {
	err := tdtp.Delegate.WriteBinary(ctx, value)
	tdtp.DuplicateTo.WriteBinary(ctx, value)
	return err
}

func (tdtp *tDuplicateToProtocol) ReadMessageBegin(ctx context.Context) (name string, typeID thrift.TMessageType, seqID int32, err error) {
	name, typeID, seqID, err = tdtp.Delegate.ReadMessageBegin(ctx)
	tdtp.DuplicateTo.WriteMessageBegin(ctx, name, typeID, seqID)
	return
}

func (tdtp *tDuplicateToProtocol) ReadMessageEnd(ctx context.Context) (err error) {
	err = tdtp.Delegate.ReadMessageEnd(ctx)
	tdtp.DuplicateTo.WriteMessageEnd(ctx)
	return
}

func (tdtp *tDuplicateToProtocol) ReadStructBegin(ctx context.Context) (name string, err error) {
	name, err = tdtp.Delegate.ReadStructBegin(ctx)
	tdtp.DuplicateTo.WriteStructBegin(ctx, name)
	return
}

func (tdtp *tDuplicateToProtocol) ReadStructEnd(ctx context.Context) (err error) {
	err = tdtp.Delegate.ReadStructEnd(ctx)
	tdtp.DuplicateTo.WriteStructEnd(ctx)
	return
}

func (tdtp *tDuplicateToProtocol) ReadFieldBegin(ctx context.Context) (name string, typeID thrift.TType, id int16, err error) {
	name, typeID, id, err = tdtp.Delegate.ReadFieldBegin(ctx)
	tdtp.DuplicateTo.WriteFieldBegin(ctx, name, typeID, id)
	return
}

func (tdtp *tDuplicateToProtocol) ReadFieldEnd(ctx context.Context) (err error) {
	err = tdtp.Delegate.ReadFieldEnd(ctx)
	tdtp.DuplicateTo.WriteFieldEnd(ctx)
	return
}

func (tdtp *tDuplicateToProtocol) ReadMapBegin(ctx context.Context) (keyType thrift.TType, valueType thrift.TType, size int, err error) {
	keyType, valueType, size, err = tdtp.Delegate.ReadMapBegin(ctx)
	tdtp.DuplicateTo.WriteMapBegin(ctx, keyType, valueType, size)
	return
}

func (tdtp *tDuplicateToProtocol) ReadMapEnd(ctx context.Context) (err error) {
	err = tdtp.Delegate.ReadMapEnd(ctx)
	tdtp.DuplicateTo.WriteMapEnd(ctx)
	return
}

func (tdtp *tDuplicateToProtocol) ReadListBegin(ctx context.Context) (elemType thrift.TType, size int, err error) {
	elemType, size, err = tdtp.Delegate.ReadListBegin(ctx)
	tdtp.DuplicateTo.WriteListBegin(ctx, elemType, size)
	return
}

func (tdtp *tDuplicateToProtocol) ReadListEnd(ctx context.Context) (err error) {
	err = tdtp.Delegate.ReadListEnd(ctx)
	tdtp.DuplicateTo.WriteListEnd(ctx)
	return
}

func (tdtp *tDuplicateToProtocol) ReadSetBegin(ctx context.Context) (elemType thrift.TType, size int, err error) {
	elemType, size, err = tdtp.Delegate.ReadSetBegin(ctx)
	tdtp.DuplicateTo.WriteSetBegin(ctx, elemType, size)
	return
}

func (tdtp *tDuplicateToProtocol) ReadSetEnd(ctx context.Context) (err error) {
	err = tdtp.Delegate.ReadSetEnd(ctx)
	tdtp.DuplicateTo.WriteSetEnd(ctx)
	return
}

func (tdtp *tDuplicateToProtocol) ReadBool(ctx context.Context) (value bool, err error) {
	value, err = tdtp.Delegate.ReadBool(ctx)
	tdtp.DuplicateTo.WriteBool(ctx, value)
	return
}

func (tdtp *tDuplicateToProtocol) ReadByte(ctx context.Context) (value int8, err error) {
	value, err = tdtp.Delegate.ReadByte(ctx)
	tdtp.DuplicateTo.WriteByte(ctx, value)
	return
}

func (tdtp *tDuplicateToProtocol) ReadI16(ctx context.Context) (value int16, err error) {
	value, err = tdtp.Delegate.ReadI16(ctx)
	tdtp.DuplicateTo.WriteI16(ctx, value)
	return
}

func (tdtp *tDuplicateToProtocol) ReadI32(ctx context.Context) (value int32, err error) {
	value, err = tdtp.Delegate.ReadI32(ctx)
	tdtp.DuplicateTo.WriteI32(ctx, value)
	return
}

func (tdtp *tDuplicateToProtocol) ReadI64(ctx context.Context) (value int64, err error) {
	value, err = tdtp.Delegate.ReadI64(ctx)
	tdtp.DuplicateTo.WriteI64(ctx, value)
	return
}

func (tdtp *tDuplicateToProtocol) ReadDouble(ctx context.Context) (value float64, err error) {
	value, err = tdtp.Delegate.ReadDouble(ctx)
	tdtp.DuplicateTo.WriteDouble(ctx, value)
	return
}

func (tdtp *tDuplicateToProtocol) ReadString(ctx context.Context) (value string, err error) {
	value, err = tdtp.Delegate.ReadString(ctx)
	tdtp.DuplicateTo.WriteString(ctx, value)
	return
}

func (tdtp *tDuplicateToProtocol) ReadBinary(ctx context.Context) (value []byte, err error) {
	value, err = tdtp.Delegate.ReadBinary(ctx)
	tdtp.DuplicateTo.WriteBinary(ctx, value)
	return
}

func (tdtp *tDuplicateToProtocol) Skip(ctx context.Context, fieldType thrift.TType) (err error) {
	err = tdtp.Delegate.Skip(ctx, fieldType)
	tdtp.DuplicateTo.Skip(ctx, fieldType)
	return
}

func (tdtp *tDuplicateToProtocol) Flush(ctx context.Context) (err error) {
	err = tdtp.Delegate.Flush(ctx)
	tdtp.DuplicateTo.Flush(ctx)
	return
}

func (tdtp *tDuplicateToProtocol) Transport() thrift.TTransport {
	return tdtp.Delegate.Transport()
}

// SetTConfiguration implements TConfigurationSetter for propagation.
func (tdtp *tDuplicateToProtocol) SetTConfiguration(conf *thrift.TConfiguration) {
	thrift.PropagateTConfiguration(tdtp.Delegate, conf)
	thrift.PropagateTConfiguration(tdtp.DuplicateTo, conf)
}

var (
	_ thrift.TConfigurationSetter = (*tDuplicateToProtocol)(nil)
	_ thrift.TProtocol            = (*tDuplicateToProtocol)(nil)
)
