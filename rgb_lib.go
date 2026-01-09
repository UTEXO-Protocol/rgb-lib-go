package rgb_lib

/*
#cgo LDFLAGS: -lrgblibuniffi -L${SRCDIR}/lib -Wl,-rpath,${SRCDIR}/lib
*/

// #include <rgb_lib.h>
import "C"

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"runtime"
	"sync/atomic"
	"unsafe"
)

// This is needed, because as of go 1.24
// type RustBuffer C.RustBuffer cannot have methods,
// RustBuffer is treated as non-local type
type GoRustBuffer struct {
	inner C.RustBuffer
}

type RustBufferI interface {
	AsReader() *bytes.Reader
	Free()
	ToGoBytes() []byte
	Data() unsafe.Pointer
	Len() uint64
	Capacity() uint64
}

func RustBufferFromExternal(b RustBufferI) GoRustBuffer {
	return GoRustBuffer{
		inner: C.RustBuffer{
			capacity: C.uint64_t(b.Capacity()),
			len:      C.uint64_t(b.Len()),
			data:     (*C.uchar)(b.Data()),
		},
	}
}

func (cb GoRustBuffer) Capacity() uint64 {
	return uint64(cb.inner.capacity)
}

func (cb GoRustBuffer) Len() uint64 {
	return uint64(cb.inner.len)
}

func (cb GoRustBuffer) Data() unsafe.Pointer {
	return unsafe.Pointer(cb.inner.data)
}

func (cb GoRustBuffer) AsReader() *bytes.Reader {
	b := unsafe.Slice((*byte)(cb.inner.data), C.uint64_t(cb.inner.len))
	return bytes.NewReader(b)
}

func (cb GoRustBuffer) Free() {
	rustCall(func(status *C.RustCallStatus) bool {
		C.ffi_rgblibuniffi_rustbuffer_free(cb.inner, status)
		return false
	})
}

func (cb GoRustBuffer) ToGoBytes() []byte {
	return C.GoBytes(unsafe.Pointer(cb.inner.data), C.int(cb.inner.len))
}

func stringToRustBuffer(str string) C.RustBuffer {
	return bytesToRustBuffer([]byte(str))
}

func bytesToRustBuffer(b []byte) C.RustBuffer {
	if len(b) == 0 {
		return C.RustBuffer{}
	}
	// We can pass the pointer along here, as it is pinned
	// for the duration of this call
	foreign := C.ForeignBytes{
		len:  C.int(len(b)),
		data: (*C.uchar)(unsafe.Pointer(&b[0])),
	}

	return rustCall(func(status *C.RustCallStatus) C.RustBuffer {
		return C.ffi_rgblibuniffi_rustbuffer_from_bytes(foreign, status)
	})
}

type BufLifter[GoType any] interface {
	Lift(value RustBufferI) GoType
}

type BufLowerer[GoType any] interface {
	Lower(value GoType) C.RustBuffer
}

type BufReader[GoType any] interface {
	Read(reader io.Reader) GoType
}

type BufWriter[GoType any] interface {
	Write(writer io.Writer, value GoType)
}

func LowerIntoRustBuffer[GoType any](bufWriter BufWriter[GoType], value GoType) C.RustBuffer {
	// This might be not the most efficient way but it does not require knowing allocation size
	// beforehand
	var buffer bytes.Buffer
	bufWriter.Write(&buffer, value)

	bytes, err := io.ReadAll(&buffer)
	if err != nil {
		panic(fmt.Errorf("reading written data: %w", err))
	}
	return bytesToRustBuffer(bytes)
}

func LiftFromRustBuffer[GoType any](bufReader BufReader[GoType], rbuf RustBufferI) GoType {
	defer rbuf.Free()
	reader := rbuf.AsReader()
	item := bufReader.Read(reader)
	if reader.Len() > 0 {
		// TODO: Remove this
		leftover, _ := io.ReadAll(reader)
		panic(fmt.Errorf("Junk remaining in buffer after lifting: %s", string(leftover)))
	}
	return item
}

func rustCallWithError[E any, U any](converter BufReader[*E], callback func(*C.RustCallStatus) U) (U, *E) {
	var status C.RustCallStatus
	returnValue := callback(&status)
	err := checkCallStatus(converter, status)
	return returnValue, err
}

func checkCallStatus[E any](converter BufReader[*E], status C.RustCallStatus) *E {
	switch status.code {
	case 0:
		return nil
	case 1:
		return LiftFromRustBuffer(converter, GoRustBuffer{inner: status.errorBuf})
	case 2:
		// when the rust code sees a panic, it tries to construct a rustBuffer
		// with the message.  but if that code panics, then it just sends back
		// an empty buffer.
		if status.errorBuf.len > 0 {
			panic(fmt.Errorf("%s", FfiConverterStringINSTANCE.Lift(GoRustBuffer{inner: status.errorBuf})))
		} else {
			panic(fmt.Errorf("Rust panicked while handling Rust panic"))
		}
	default:
		panic(fmt.Errorf("unknown status code: %d", status.code))
	}
}

func checkCallStatusUnknown(status C.RustCallStatus) error {
	switch status.code {
	case 0:
		return nil
	case 1:
		panic(fmt.Errorf("function not returning an error returned an error"))
	case 2:
		// when the rust code sees a panic, it tries to construct a C.RustBuffer
		// with the message.  but if that code panics, then it just sends back
		// an empty buffer.
		if status.errorBuf.len > 0 {
			panic(fmt.Errorf("%s", FfiConverterStringINSTANCE.Lift(GoRustBuffer{
				inner: status.errorBuf,
			})))
		} else {
			panic(fmt.Errorf("Rust panicked while handling Rust panic"))
		}
	default:
		return fmt.Errorf("unknown status code: %d", status.code)
	}
}

func rustCall[U any](callback func(*C.RustCallStatus) U) U {
	returnValue, err := rustCallWithError[error](nil, callback)
	if err != nil {
		panic(err)
	}
	return returnValue
}

type NativeError interface {
	AsError() error
}

func writeInt8(writer io.Writer, value int8) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint8(writer io.Writer, value uint8) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt16(writer io.Writer, value int16) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint16(writer io.Writer, value uint16) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt32(writer io.Writer, value int32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint32(writer io.Writer, value uint32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt64(writer io.Writer, value int64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint64(writer io.Writer, value uint64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeFloat32(writer io.Writer, value float32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeFloat64(writer io.Writer, value float64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func readInt8(reader io.Reader) int8 {
	var result int8
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint8(reader io.Reader) uint8 {
	var result uint8
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt16(reader io.Reader) int16 {
	var result int16
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint16(reader io.Reader) uint16 {
	var result uint16
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt32(reader io.Reader) int32 {
	var result int32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint32(reader io.Reader) uint32 {
	var result uint32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt64(reader io.Reader) int64 {
	var result int64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint64(reader io.Reader) uint64 {
	var result uint64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readFloat32(reader io.Reader) float32 {
	var result float32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readFloat64(reader io.Reader) float64 {
	var result float64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func init() {

	uniffiCheckChecksums()
}

func uniffiCheckChecksums() {
	// Get the bindings contract version from our ComponentInterface
	bindingsContractVersion := 29
	// Get the scaffolding contract version by calling the into the dylib
	scaffoldingContractVersion := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.ffi_rgblibuniffi_uniffi_contract_version()
	})
	if bindingsContractVersion != int(scaffoldingContractVersion) {
		// If this happens try cleaning and rebuilding your project
		panic("rgb_lib: UniFFI contract version mismatch")
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_func_generate_keys()
		})
		if checksum != 50781 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_func_generate_keys: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_func_restore_backup()
		})
		if checksum != 4743 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_func_restore_backup: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_func_restore_keys()
		})
		if checksum != 38408 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_func_restore_keys: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_invoice_invoice_data()
		})
		if checksum != 31294 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_invoice_invoice_data: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_invoice_invoice_string()
		})
		if checksum != 25144 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_invoice_invoice_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_recipientinfo_network()
		})
		if checksum != 22005 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_recipientinfo_network: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_recipientinfo_recipient_type()
		})
		if checksum != 3457 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_recipientinfo_recipient_type: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_transportendpoint_transport_type()
		})
		if checksum != 33510 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_transportendpoint_transport_type: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_backup()
		})
		if checksum != 41851 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_backup: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_backup_info()
		})
		if checksum != 7253 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_backup_info: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_blind_receive()
		})
		if checksum != 51838 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_blind_receive: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_create_utxos()
		})
		if checksum != 42058 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_create_utxos: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_create_utxos_begin()
		})
		if checksum != 30727 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_create_utxos_begin: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_create_utxos_end()
		})
		if checksum != 50137 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_create_utxos_end: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_delete_transfers()
		})
		if checksum != 43847 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_delete_transfers: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_drain_to()
		})
		if checksum != 60164 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_drain_to: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_drain_to_begin()
		})
		if checksum != 57452 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_drain_to_begin: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_drain_to_end()
		})
		if checksum != 62328 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_drain_to_end: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_fail_transfers()
		})
		if checksum != 7914 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_fail_transfers: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_finalize_psbt()
		})
		if checksum != 39319 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_finalize_psbt: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_get_address()
		})
		if checksum != 23668 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_get_address: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_get_asset_balance()
		})
		if checksum != 19662 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_get_asset_balance: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_get_asset_metadata()
		})
		if checksum != 58573 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_get_asset_metadata: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_get_btc_balance()
		})
		if checksum != 40762 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_get_btc_balance: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_get_fee_estimation()
		})
		if checksum != 64220 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_get_fee_estimation: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_get_media_dir()
		})
		if checksum != 64429 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_get_media_dir: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_get_wallet_data()
		})
		if checksum != 18071 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_get_wallet_data: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_get_wallet_dir()
		})
		if checksum != 8726 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_get_wallet_dir: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_go_online()
		})
		if checksum != 46399 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_go_online: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_issue_asset_cfa()
		})
		if checksum != 32847 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_issue_asset_cfa: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_issue_asset_ifa()
		})
		if checksum != 50556 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_issue_asset_ifa: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_issue_asset_nia()
		})
		if checksum != 54511 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_issue_asset_nia: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_issue_asset_uda()
		})
		if checksum != 63508 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_issue_asset_uda: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_list_assets()
		})
		if checksum != 18027 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_list_assets: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_list_transactions()
		})
		if checksum != 40825 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_list_transactions: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_list_transfers()
		})
		if checksum != 36530 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_list_transfers: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_list_unspents()
		})
		if checksum != 62734 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_list_unspents: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_refresh()
		})
		if checksum != 45223 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_refresh: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_send()
		})
		if checksum != 57749 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_send: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_send_begin()
		})
		if checksum != 46093 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_send_begin: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_send_btc()
		})
		if checksum != 15823 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_send_btc: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_send_btc_begin()
		})
		if checksum != 59961 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_send_btc_begin: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_send_btc_end()
		})
		if checksum != 60404 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_send_btc_end: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_send_end()
		})
		if checksum != 1754 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_send_end: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_sign_psbt()
		})
		if checksum != 10485 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_sign_psbt: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_sync()
		})
		if checksum != 22767 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_sync: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_method_wallet_witness_receive()
		})
		if checksum != 541 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_method_wallet_witness_receive: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_constructor_address_new()
		})
		if checksum != 14676 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_constructor_address_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_constructor_invoice_new()
		})
		if checksum != 33585 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_constructor_invoice_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_constructor_recipientinfo_new()
		})
		if checksum != 56664 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_constructor_recipientinfo_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_constructor_transportendpoint_new()
		})
		if checksum != 38802 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_constructor_transportendpoint_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_rgblibuniffi_checksum_constructor_wallet_new()
		})
		if checksum != 29566 {
			// If this happens try cleaning and rebuilding your project
			panic("rgb_lib: uniffi_rgblibuniffi_checksum_constructor_wallet_new: UniFFI API checksum mismatch")
		}
	}
}

type FfiConverterUint8 struct{}

var FfiConverterUint8INSTANCE = FfiConverterUint8{}

func (FfiConverterUint8) Lower(value uint8) C.uint8_t {
	return C.uint8_t(value)
}

func (FfiConverterUint8) Write(writer io.Writer, value uint8) {
	writeUint8(writer, value)
}

func (FfiConverterUint8) Lift(value C.uint8_t) uint8 {
	return uint8(value)
}

func (FfiConverterUint8) Read(reader io.Reader) uint8 {
	return readUint8(reader)
}

type FfiDestroyerUint8 struct{}

func (FfiDestroyerUint8) Destroy(_ uint8) {}

type FfiConverterUint16 struct{}

var FfiConverterUint16INSTANCE = FfiConverterUint16{}

func (FfiConverterUint16) Lower(value uint16) C.uint16_t {
	return C.uint16_t(value)
}

func (FfiConverterUint16) Write(writer io.Writer, value uint16) {
	writeUint16(writer, value)
}

func (FfiConverterUint16) Lift(value C.uint16_t) uint16 {
	return uint16(value)
}

func (FfiConverterUint16) Read(reader io.Reader) uint16 {
	return readUint16(reader)
}

type FfiDestroyerUint16 struct{}

func (FfiDestroyerUint16) Destroy(_ uint16) {}

type FfiConverterUint32 struct{}

var FfiConverterUint32INSTANCE = FfiConverterUint32{}

func (FfiConverterUint32) Lower(value uint32) C.uint32_t {
	return C.uint32_t(value)
}

func (FfiConverterUint32) Write(writer io.Writer, value uint32) {
	writeUint32(writer, value)
}

func (FfiConverterUint32) Lift(value C.uint32_t) uint32 {
	return uint32(value)
}

func (FfiConverterUint32) Read(reader io.Reader) uint32 {
	return readUint32(reader)
}

type FfiDestroyerUint32 struct{}

func (FfiDestroyerUint32) Destroy(_ uint32) {}

type FfiConverterInt32 struct{}

var FfiConverterInt32INSTANCE = FfiConverterInt32{}

func (FfiConverterInt32) Lower(value int32) C.int32_t {
	return C.int32_t(value)
}

func (FfiConverterInt32) Write(writer io.Writer, value int32) {
	writeInt32(writer, value)
}

func (FfiConverterInt32) Lift(value C.int32_t) int32 {
	return int32(value)
}

func (FfiConverterInt32) Read(reader io.Reader) int32 {
	return readInt32(reader)
}

type FfiDestroyerInt32 struct{}

func (FfiDestroyerInt32) Destroy(_ int32) {}

type FfiConverterUint64 struct{}

var FfiConverterUint64INSTANCE = FfiConverterUint64{}

func (FfiConverterUint64) Lower(value uint64) C.uint64_t {
	return C.uint64_t(value)
}

func (FfiConverterUint64) Write(writer io.Writer, value uint64) {
	writeUint64(writer, value)
}

func (FfiConverterUint64) Lift(value C.uint64_t) uint64 {
	return uint64(value)
}

func (FfiConverterUint64) Read(reader io.Reader) uint64 {
	return readUint64(reader)
}

type FfiDestroyerUint64 struct{}

func (FfiDestroyerUint64) Destroy(_ uint64) {}

type FfiConverterInt64 struct{}

var FfiConverterInt64INSTANCE = FfiConverterInt64{}

func (FfiConverterInt64) Lower(value int64) C.int64_t {
	return C.int64_t(value)
}

func (FfiConverterInt64) Write(writer io.Writer, value int64) {
	writeInt64(writer, value)
}

func (FfiConverterInt64) Lift(value C.int64_t) int64 {
	return int64(value)
}

func (FfiConverterInt64) Read(reader io.Reader) int64 {
	return readInt64(reader)
}

type FfiDestroyerInt64 struct{}

func (FfiDestroyerInt64) Destroy(_ int64) {}

type FfiConverterFloat64 struct{}

var FfiConverterFloat64INSTANCE = FfiConverterFloat64{}

func (FfiConverterFloat64) Lower(value float64) C.double {
	return C.double(value)
}

func (FfiConverterFloat64) Write(writer io.Writer, value float64) {
	writeFloat64(writer, value)
}

func (FfiConverterFloat64) Lift(value C.double) float64 {
	return float64(value)
}

func (FfiConverterFloat64) Read(reader io.Reader) float64 {
	return readFloat64(reader)
}

type FfiDestroyerFloat64 struct{}

func (FfiDestroyerFloat64) Destroy(_ float64) {}

type FfiConverterBool struct{}

var FfiConverterBoolINSTANCE = FfiConverterBool{}

func (FfiConverterBool) Lower(value bool) C.int8_t {
	if value {
		return C.int8_t(1)
	}
	return C.int8_t(0)
}

func (FfiConverterBool) Write(writer io.Writer, value bool) {
	if value {
		writeInt8(writer, 1)
	} else {
		writeInt8(writer, 0)
	}
}

func (FfiConverterBool) Lift(value C.int8_t) bool {
	return value != 0
}

func (FfiConverterBool) Read(reader io.Reader) bool {
	return readInt8(reader) != 0
}

type FfiDestroyerBool struct{}

func (FfiDestroyerBool) Destroy(_ bool) {}

type FfiConverterString struct{}

var FfiConverterStringINSTANCE = FfiConverterString{}

func (FfiConverterString) Lift(rb RustBufferI) string {
	defer rb.Free()
	reader := rb.AsReader()
	b, err := io.ReadAll(reader)
	if err != nil {
		panic(fmt.Errorf("reading reader: %w", err))
	}
	return string(b)
}

func (FfiConverterString) Read(reader io.Reader) string {
	length := readInt32(reader)
	buffer := make([]byte, length)
	read_length, err := reader.Read(buffer)
	if err != nil && err != io.EOF {
		panic(err)
	}
	if read_length != int(length) {
		panic(fmt.Errorf("bad read length when reading string, expected %d, read %d", length, read_length))
	}
	return string(buffer)
}

func (FfiConverterString) Lower(value string) C.RustBuffer {
	return stringToRustBuffer(value)
}

func (FfiConverterString) Write(writer io.Writer, value string) {
	if len(value) > math.MaxInt32 {
		panic("String is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	write_length, err := io.WriteString(writer, value)
	if err != nil {
		panic(err)
	}
	if write_length != len(value) {
		panic(fmt.Errorf("bad write length when writing string, expected %d, written %d", len(value), write_length))
	}
}

type FfiDestroyerString struct{}

func (FfiDestroyerString) Destroy(_ string) {}

// Below is an implementation of synchronization requirements outlined in the link.
// https://github.com/mozilla/uniffi-rs/blob/0dc031132d9493ca812c3af6e7dd60ad2ea95bf0/uniffi_bindgen/src/bindings/kotlin/templates/ObjectRuntime.kt#L31

type FfiObject struct {
	pointer       unsafe.Pointer
	callCounter   atomic.Int64
	cloneFunction func(unsafe.Pointer, *C.RustCallStatus) unsafe.Pointer
	freeFunction  func(unsafe.Pointer, *C.RustCallStatus)
	destroyed     atomic.Bool
}

func newFfiObject(
	pointer unsafe.Pointer,
	cloneFunction func(unsafe.Pointer, *C.RustCallStatus) unsafe.Pointer,
	freeFunction func(unsafe.Pointer, *C.RustCallStatus),
) FfiObject {
	return FfiObject{
		pointer:       pointer,
		cloneFunction: cloneFunction,
		freeFunction:  freeFunction,
	}
}

func (ffiObject *FfiObject) incrementPointer(debugName string) unsafe.Pointer {
	for {
		counter := ffiObject.callCounter.Load()
		if counter <= -1 {
			panic(fmt.Errorf("%v object has already been destroyed", debugName))
		}
		if counter == math.MaxInt64 {
			panic(fmt.Errorf("%v object call counter would overflow", debugName))
		}
		if ffiObject.callCounter.CompareAndSwap(counter, counter+1) {
			break
		}
	}

	return rustCall(func(status *C.RustCallStatus) unsafe.Pointer {
		return ffiObject.cloneFunction(ffiObject.pointer, status)
	})
}

func (ffiObject *FfiObject) decrementPointer() {
	if ffiObject.callCounter.Add(-1) == -1 {
		ffiObject.freeRustArcPtr()
	}
}

func (ffiObject *FfiObject) destroy() {
	if ffiObject.destroyed.CompareAndSwap(false, true) {
		if ffiObject.callCounter.Add(-1) == -1 {
			ffiObject.freeRustArcPtr()
		}
	}
}

func (ffiObject *FfiObject) freeRustArcPtr() {
	rustCall(func(status *C.RustCallStatus) int32 {
		ffiObject.freeFunction(ffiObject.pointer, status)
		return 0
	})
}

type AddressInterface interface {
}
type Address struct {
	ffiObject FfiObject
}

func NewAddress(addressString string, bitcoinNetwork BitcoinNetwork) (*Address, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_rgblibuniffi_fn_constructor_address_new(FfiConverterStringINSTANCE.Lower(addressString), FfiConverterBitcoinNetworkINSTANCE.Lower(bitcoinNetwork), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Address
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterAddressINSTANCE.Lift(_uniffiRV), nil
	}
}

func (object *Address) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterAddress struct{}

var FfiConverterAddressINSTANCE = FfiConverterAddress{}

func (c FfiConverterAddress) Lift(pointer unsafe.Pointer) *Address {
	result := &Address{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_rgblibuniffi_fn_clone_address(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_rgblibuniffi_fn_free_address(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Address).Destroy)
	return result
}

func (c FfiConverterAddress) Read(reader io.Reader) *Address {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterAddress) Lower(value *Address) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Address")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterAddress) Write(writer io.Writer, value *Address) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerAddress struct{}

func (_ FfiDestroyerAddress) Destroy(value *Address) {
	value.Destroy()
}

type InvoiceInterface interface {
	InvoiceData() InvoiceData
	InvoiceString() string
}
type Invoice struct {
	ffiObject FfiObject
}

func NewInvoice(invoiceString string) (*Invoice, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_rgblibuniffi_fn_constructor_invoice_new(FfiConverterStringINSTANCE.Lower(invoiceString), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Invoice
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterInvoiceINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Invoice) InvoiceData() InvoiceData {
	_pointer := _self.ffiObject.incrementPointer("*Invoice")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterInvoiceDataINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_invoice_invoice_data(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Invoice) InvoiceString() string {
	_pointer := _self.ffiObject.incrementPointer("*Invoice")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_invoice_invoice_string(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *Invoice) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterInvoice struct{}

var FfiConverterInvoiceINSTANCE = FfiConverterInvoice{}

func (c FfiConverterInvoice) Lift(pointer unsafe.Pointer) *Invoice {
	result := &Invoice{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_rgblibuniffi_fn_clone_invoice(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_rgblibuniffi_fn_free_invoice(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Invoice).Destroy)
	return result
}

func (c FfiConverterInvoice) Read(reader io.Reader) *Invoice {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterInvoice) Lower(value *Invoice) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Invoice")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterInvoice) Write(writer io.Writer, value *Invoice) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerInvoice struct{}

func (_ FfiDestroyerInvoice) Destroy(value *Invoice) {
	value.Destroy()
}

type RecipientInfoInterface interface {
	Network() BitcoinNetwork
	RecipientType() RecipientType
}
type RecipientInfo struct {
	ffiObject FfiObject
}

func NewRecipientInfo(recipientId string) (*RecipientInfo, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_rgblibuniffi_fn_constructor_recipientinfo_new(FfiConverterStringINSTANCE.Lower(recipientId), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *RecipientInfo
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterRecipientInfoINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *RecipientInfo) Network() BitcoinNetwork {
	_pointer := _self.ffiObject.incrementPointer("*RecipientInfo")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBitcoinNetworkINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_recipientinfo_network(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *RecipientInfo) RecipientType() RecipientType {
	_pointer := _self.ffiObject.incrementPointer("*RecipientInfo")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterRecipientTypeINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_recipientinfo_recipient_type(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *RecipientInfo) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterRecipientInfo struct{}

var FfiConverterRecipientInfoINSTANCE = FfiConverterRecipientInfo{}

func (c FfiConverterRecipientInfo) Lift(pointer unsafe.Pointer) *RecipientInfo {
	result := &RecipientInfo{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_rgblibuniffi_fn_clone_recipientinfo(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_rgblibuniffi_fn_free_recipientinfo(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*RecipientInfo).Destroy)
	return result
}

func (c FfiConverterRecipientInfo) Read(reader io.Reader) *RecipientInfo {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterRecipientInfo) Lower(value *RecipientInfo) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*RecipientInfo")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterRecipientInfo) Write(writer io.Writer, value *RecipientInfo) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerRecipientInfo struct{}

func (_ FfiDestroyerRecipientInfo) Destroy(value *RecipientInfo) {
	value.Destroy()
}

type TransportEndpointInterface interface {
	TransportType() TransportType
}
type TransportEndpoint struct {
	ffiObject FfiObject
}

func NewTransportEndpoint(transportEndpoint string) (*TransportEndpoint, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_rgblibuniffi_fn_constructor_transportendpoint_new(FfiConverterStringINSTANCE.Lower(transportEndpoint), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *TransportEndpoint
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTransportEndpointINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *TransportEndpoint) TransportType() TransportType {
	_pointer := _self.ffiObject.incrementPointer("*TransportEndpoint")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTransportTypeINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_transportendpoint_transport_type(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *TransportEndpoint) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterTransportEndpoint struct{}

var FfiConverterTransportEndpointINSTANCE = FfiConverterTransportEndpoint{}

func (c FfiConverterTransportEndpoint) Lift(pointer unsafe.Pointer) *TransportEndpoint {
	result := &TransportEndpoint{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_rgblibuniffi_fn_clone_transportendpoint(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_rgblibuniffi_fn_free_transportendpoint(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*TransportEndpoint).Destroy)
	return result
}

func (c FfiConverterTransportEndpoint) Read(reader io.Reader) *TransportEndpoint {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterTransportEndpoint) Lower(value *TransportEndpoint) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*TransportEndpoint")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterTransportEndpoint) Write(writer io.Writer, value *TransportEndpoint) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerTransportEndpoint struct{}

func (_ FfiDestroyerTransportEndpoint) Destroy(value *TransportEndpoint) {
	value.Destroy()
}

type WalletInterface interface {
	Backup(backupPath string, password string) error
	BackupInfo() (bool, error)
	BlindReceive(assetId *string, assignment Assignment, durationSeconds *uint32, transportEndpoints []string, minConfirmations uint8) (ReceiveData, error)
	CreateUtxos(online Online, upTo bool, num *uint8, size *uint32, feeRate uint64, skipSync bool) (uint8, error)
	CreateUtxosBegin(online Online, upTo bool, num *uint8, size *uint32, feeRate uint64, skipSync bool) (string, error)
	CreateUtxosEnd(online Online, signedPsbt string, skipSync bool) (uint8, error)
	DeleteTransfers(batchTransferIdx *int32, noAssetOnly bool) (bool, error)
	DrainTo(online Online, address string, destroyAssets bool, feeRate uint64) (string, error)
	DrainToBegin(online Online, address string, destroyAssets bool, feeRate uint64) (string, error)
	DrainToEnd(online Online, signedPsbt string) (string, error)
	FailTransfers(online Online, batchTransferIdx *int32, noAssetOnly bool, skipSync bool) (bool, error)
	FinalizePsbt(signedPsbt string) (string, error)
	GetAddress() (string, error)
	GetAssetBalance(assetId string) (Balance, error)
	GetAssetMetadata(assetId string) (Metadata, error)
	GetBtcBalance(online *Online, skipSync bool) (BtcBalance, error)
	GetFeeEstimation(online Online, blocks uint16) (float64, error)
	GetMediaDir() string
	GetWalletData() WalletData
	GetWalletDir() string
	GoOnline(skipConsistencyCheck bool, indexerUrl string) (Online, error)
	IssueAssetCfa(name string, details *string, precision uint8, amounts []uint64, filePath *string) (AssetCfa, error)
	IssueAssetIfa(ticker string, name string, precision uint8, amounts []uint64, inflationAmounts []uint64, replaceRightsNum uint8) (AssetIfa, error)
	IssueAssetNia(ticker string, name string, precision uint8, amounts []uint64) (AssetNia, error)
	IssueAssetUda(ticker string, name string, details *string, precision uint8, mediaFilePath *string, attachmentsFilePaths []string) (AssetUda, error)
	ListAssets(filterAssetSchemas []AssetSchema) (Assets, error)
	ListTransactions(online *Online, skipSync bool) ([]Transaction, error)
	ListTransfers(assetId *string) ([]Transfer, error)
	ListUnspents(online *Online, settledOnly bool, skipSync bool) ([]Unspent, error)
	Refresh(online Online, assetId *string, filter []RefreshFilter, skipSync bool) (map[int32]RefreshedTransfer, error)
	Send(online Online, recipientMap map[string][]Recipient, donation bool, feeRate uint64, minConfirmations uint8, skipSync bool) (SendResult, error)
	SendBegin(online Online, recipientMap map[string][]Recipient, donation bool, feeRate uint64, minConfirmations uint8) (string, error)
	SendBtc(online Online, address string, amount uint64, feeRate uint64, skipSync bool) (string, error)
	SendBtcBegin(online Online, address string, amount uint64, feeRate uint64, skipSync bool) (string, error)
	SendBtcEnd(online Online, signedPsbt string, skipSync bool) (string, error)
	SendEnd(online Online, signedPsbt string, skipSync bool) (SendResult, error)
	SignPsbt(unsignedPsbt string) (string, error)
	Sync(online Online) error
	WitnessReceive(assetId *string, assignment Assignment, durationSeconds *uint32, transportEndpoints []string, minConfirmations uint8) (ReceiveData, error)
}
type Wallet struct {
	ffiObject FfiObject
}

func NewWallet(walletData WalletData) (*Wallet, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_rgblibuniffi_fn_constructor_wallet_new(FfiConverterWalletDataINSTANCE.Lower(walletData), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Wallet
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterWalletINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) Backup(backupPath string, password string) error {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_rgblibuniffi_fn_method_wallet_backup(
			_pointer, FfiConverterStringINSTANCE.Lower(backupPath), FfiConverterStringINSTANCE.Lower(password), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

func (_self *Wallet) BackupInfo() (bool, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_rgblibuniffi_fn_method_wallet_backup_info(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue bool
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBoolINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) BlindReceive(assetId *string, assignment Assignment, durationSeconds *uint32, transportEndpoints []string, minConfirmations uint8) (ReceiveData, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_blind_receive(
				_pointer, FfiConverterOptionalStringINSTANCE.Lower(assetId), FfiConverterAssignmentINSTANCE.Lower(assignment), FfiConverterOptionalUint32INSTANCE.Lower(durationSeconds), FfiConverterSequenceStringINSTANCE.Lower(transportEndpoints), FfiConverterUint8INSTANCE.Lower(minConfirmations), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue ReceiveData
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterReceiveDataINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) CreateUtxos(online Online, upTo bool, num *uint8, size *uint32, feeRate uint64, skipSync bool) (uint8, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) C.uint8_t {
		return C.uniffi_rgblibuniffi_fn_method_wallet_create_utxos(
			_pointer, FfiConverterOnlineINSTANCE.Lower(online), FfiConverterBoolINSTANCE.Lower(upTo), FfiConverterOptionalUint8INSTANCE.Lower(num), FfiConverterOptionalUint32INSTANCE.Lower(size), FfiConverterUint64INSTANCE.Lower(feeRate), FfiConverterBoolINSTANCE.Lower(skipSync), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue uint8
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterUint8INSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) CreateUtxosBegin(online Online, upTo bool, num *uint8, size *uint32, feeRate uint64, skipSync bool) (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_create_utxos_begin(
				_pointer, FfiConverterOnlineINSTANCE.Lower(online), FfiConverterBoolINSTANCE.Lower(upTo), FfiConverterOptionalUint8INSTANCE.Lower(num), FfiConverterOptionalUint32INSTANCE.Lower(size), FfiConverterUint64INSTANCE.Lower(feeRate), FfiConverterBoolINSTANCE.Lower(skipSync), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) CreateUtxosEnd(online Online, signedPsbt string, skipSync bool) (uint8, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) C.uint8_t {
		return C.uniffi_rgblibuniffi_fn_method_wallet_create_utxos_end(
			_pointer, FfiConverterOnlineINSTANCE.Lower(online), FfiConverterStringINSTANCE.Lower(signedPsbt), FfiConverterBoolINSTANCE.Lower(skipSync), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue uint8
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterUint8INSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) DeleteTransfers(batchTransferIdx *int32, noAssetOnly bool) (bool, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_rgblibuniffi_fn_method_wallet_delete_transfers(
			_pointer, FfiConverterOptionalInt32INSTANCE.Lower(batchTransferIdx), FfiConverterBoolINSTANCE.Lower(noAssetOnly), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue bool
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBoolINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) DrainTo(online Online, address string, destroyAssets bool, feeRate uint64) (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_drain_to(
				_pointer, FfiConverterOnlineINSTANCE.Lower(online), FfiConverterStringINSTANCE.Lower(address), FfiConverterBoolINSTANCE.Lower(destroyAssets), FfiConverterUint64INSTANCE.Lower(feeRate), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) DrainToBegin(online Online, address string, destroyAssets bool, feeRate uint64) (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_drain_to_begin(
				_pointer, FfiConverterOnlineINSTANCE.Lower(online), FfiConverterStringINSTANCE.Lower(address), FfiConverterBoolINSTANCE.Lower(destroyAssets), FfiConverterUint64INSTANCE.Lower(feeRate), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) DrainToEnd(online Online, signedPsbt string) (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_drain_to_end(
				_pointer, FfiConverterOnlineINSTANCE.Lower(online), FfiConverterStringINSTANCE.Lower(signedPsbt), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) FailTransfers(online Online, batchTransferIdx *int32, noAssetOnly bool, skipSync bool) (bool, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_rgblibuniffi_fn_method_wallet_fail_transfers(
			_pointer, FfiConverterOnlineINSTANCE.Lower(online), FfiConverterOptionalInt32INSTANCE.Lower(batchTransferIdx), FfiConverterBoolINSTANCE.Lower(noAssetOnly), FfiConverterBoolINSTANCE.Lower(skipSync), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue bool
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBoolINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) FinalizePsbt(signedPsbt string) (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_finalize_psbt(
				_pointer, FfiConverterStringINSTANCE.Lower(signedPsbt), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) GetAddress() (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_get_address(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) GetAssetBalance(assetId string) (Balance, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_get_asset_balance(
				_pointer, FfiConverterStringINSTANCE.Lower(assetId), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue Balance
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBalanceINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) GetAssetMetadata(assetId string) (Metadata, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_get_asset_metadata(
				_pointer, FfiConverterStringINSTANCE.Lower(assetId), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue Metadata
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterMetadataINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) GetBtcBalance(online *Online, skipSync bool) (BtcBalance, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_get_btc_balance(
				_pointer, FfiConverterOptionalOnlineINSTANCE.Lower(online), FfiConverterBoolINSTANCE.Lower(skipSync), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue BtcBalance
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBtcBalanceINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) GetFeeEstimation(online Online, blocks uint16) (float64, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) C.double {
		return C.uniffi_rgblibuniffi_fn_method_wallet_get_fee_estimation(
			_pointer, FfiConverterOnlineINSTANCE.Lower(online), FfiConverterUint16INSTANCE.Lower(blocks), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue float64
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterFloat64INSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) GetMediaDir() string {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_get_media_dir(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Wallet) GetWalletData() WalletData {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterWalletDataINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_get_wallet_data(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Wallet) GetWalletDir() string {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_get_wallet_dir(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Wallet) GoOnline(skipConsistencyCheck bool, indexerUrl string) (Online, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_go_online(
				_pointer, FfiConverterBoolINSTANCE.Lower(skipConsistencyCheck), FfiConverterStringINSTANCE.Lower(indexerUrl), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue Online
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOnlineINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) IssueAssetCfa(name string, details *string, precision uint8, amounts []uint64, filePath *string) (AssetCfa, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_issue_asset_cfa(
				_pointer, FfiConverterStringINSTANCE.Lower(name), FfiConverterOptionalStringINSTANCE.Lower(details), FfiConverterUint8INSTANCE.Lower(precision), FfiConverterSequenceUint64INSTANCE.Lower(amounts), FfiConverterOptionalStringINSTANCE.Lower(filePath), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue AssetCfa
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterAssetCfaINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) IssueAssetIfa(ticker string, name string, precision uint8, amounts []uint64, inflationAmounts []uint64, replaceRightsNum uint8) (AssetIfa, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_issue_asset_ifa(
				_pointer, FfiConverterStringINSTANCE.Lower(ticker), FfiConverterStringINSTANCE.Lower(name), FfiConverterUint8INSTANCE.Lower(precision), FfiConverterSequenceUint64INSTANCE.Lower(amounts), FfiConverterSequenceUint64INSTANCE.Lower(inflationAmounts), FfiConverterUint8INSTANCE.Lower(replaceRightsNum), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue AssetIfa
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterAssetIfaINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) IssueAssetNia(ticker string, name string, precision uint8, amounts []uint64) (AssetNia, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_issue_asset_nia(
				_pointer, FfiConverterStringINSTANCE.Lower(ticker), FfiConverterStringINSTANCE.Lower(name), FfiConverterUint8INSTANCE.Lower(precision), FfiConverterSequenceUint64INSTANCE.Lower(amounts), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue AssetNia
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterAssetNiaINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) IssueAssetUda(ticker string, name string, details *string, precision uint8, mediaFilePath *string, attachmentsFilePaths []string) (AssetUda, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_issue_asset_uda(
				_pointer, FfiConverterStringINSTANCE.Lower(ticker), FfiConverterStringINSTANCE.Lower(name), FfiConverterOptionalStringINSTANCE.Lower(details), FfiConverterUint8INSTANCE.Lower(precision), FfiConverterOptionalStringINSTANCE.Lower(mediaFilePath), FfiConverterSequenceStringINSTANCE.Lower(attachmentsFilePaths), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue AssetUda
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterAssetUdaINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) ListAssets(filterAssetSchemas []AssetSchema) (Assets, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_list_assets(
				_pointer, FfiConverterSequenceAssetSchemaINSTANCE.Lower(filterAssetSchemas), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue Assets
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterAssetsINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) ListTransactions(online *Online, skipSync bool) ([]Transaction, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_list_transactions(
				_pointer, FfiConverterOptionalOnlineINSTANCE.Lower(online), FfiConverterBoolINSTANCE.Lower(skipSync), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []Transaction
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceTransactionINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) ListTransfers(assetId *string) ([]Transfer, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_list_transfers(
				_pointer, FfiConverterOptionalStringINSTANCE.Lower(assetId), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []Transfer
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceTransferINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) ListUnspents(online *Online, settledOnly bool, skipSync bool) ([]Unspent, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_list_unspents(
				_pointer, FfiConverterOptionalOnlineINSTANCE.Lower(online), FfiConverterBoolINSTANCE.Lower(settledOnly), FfiConverterBoolINSTANCE.Lower(skipSync), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []Unspent
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceUnspentINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) Refresh(online Online, assetId *string, filter []RefreshFilter, skipSync bool) (map[int32]RefreshedTransfer, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_refresh(
				_pointer, FfiConverterOnlineINSTANCE.Lower(online), FfiConverterOptionalStringINSTANCE.Lower(assetId), FfiConverterSequenceRefreshFilterINSTANCE.Lower(filter), FfiConverterBoolINSTANCE.Lower(skipSync), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue map[int32]RefreshedTransfer
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterMapInt32RefreshedTransferINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) Send(online Online, recipientMap map[string][]Recipient, donation bool, feeRate uint64, minConfirmations uint8, skipSync bool) (SendResult, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_send(
				_pointer, FfiConverterOnlineINSTANCE.Lower(online), FfiConverterMapStringSequenceRecipientINSTANCE.Lower(recipientMap), FfiConverterBoolINSTANCE.Lower(donation), FfiConverterUint64INSTANCE.Lower(feeRate), FfiConverterUint8INSTANCE.Lower(minConfirmations), FfiConverterBoolINSTANCE.Lower(skipSync), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue SendResult
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSendResultINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) SendBegin(online Online, recipientMap map[string][]Recipient, donation bool, feeRate uint64, minConfirmations uint8) (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_send_begin(
				_pointer, FfiConverterOnlineINSTANCE.Lower(online), FfiConverterMapStringSequenceRecipientINSTANCE.Lower(recipientMap), FfiConverterBoolINSTANCE.Lower(donation), FfiConverterUint64INSTANCE.Lower(feeRate), FfiConverterUint8INSTANCE.Lower(minConfirmations), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) SendBtc(online Online, address string, amount uint64, feeRate uint64, skipSync bool) (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_send_btc(
				_pointer, FfiConverterOnlineINSTANCE.Lower(online), FfiConverterStringINSTANCE.Lower(address), FfiConverterUint64INSTANCE.Lower(amount), FfiConverterUint64INSTANCE.Lower(feeRate), FfiConverterBoolINSTANCE.Lower(skipSync), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) SendBtcBegin(online Online, address string, amount uint64, feeRate uint64, skipSync bool) (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_send_btc_begin(
				_pointer, FfiConverterOnlineINSTANCE.Lower(online), FfiConverterStringINSTANCE.Lower(address), FfiConverterUint64INSTANCE.Lower(amount), FfiConverterUint64INSTANCE.Lower(feeRate), FfiConverterBoolINSTANCE.Lower(skipSync), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) SendBtcEnd(online Online, signedPsbt string, skipSync bool) (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_send_btc_end(
				_pointer, FfiConverterOnlineINSTANCE.Lower(online), FfiConverterStringINSTANCE.Lower(signedPsbt), FfiConverterBoolINSTANCE.Lower(skipSync), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) SendEnd(online Online, signedPsbt string, skipSync bool) (SendResult, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_send_end(
				_pointer, FfiConverterOnlineINSTANCE.Lower(online), FfiConverterStringINSTANCE.Lower(signedPsbt), FfiConverterBoolINSTANCE.Lower(skipSync), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue SendResult
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSendResultINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) SignPsbt(unsignedPsbt string) (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_sign_psbt(
				_pointer, FfiConverterStringINSTANCE.Lower(unsignedPsbt), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) Sync(online Online) error {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_rgblibuniffi_fn_method_wallet_sync(
			_pointer, FfiConverterOnlineINSTANCE.Lower(online), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

func (_self *Wallet) WitnessReceive(assetId *string, assignment Assignment, durationSeconds *uint32, transportEndpoints []string, minConfirmations uint8) (ReceiveData, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_method_wallet_witness_receive(
				_pointer, FfiConverterOptionalStringINSTANCE.Lower(assetId), FfiConverterAssignmentINSTANCE.Lower(assignment), FfiConverterOptionalUint32INSTANCE.Lower(durationSeconds), FfiConverterSequenceStringINSTANCE.Lower(transportEndpoints), FfiConverterUint8INSTANCE.Lower(minConfirmations), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue ReceiveData
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterReceiveDataINSTANCE.Lift(_uniffiRV), nil
	}
}
func (object *Wallet) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterWallet struct{}

var FfiConverterWalletINSTANCE = FfiConverterWallet{}

func (c FfiConverterWallet) Lift(pointer unsafe.Pointer) *Wallet {
	result := &Wallet{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_rgblibuniffi_fn_clone_wallet(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_rgblibuniffi_fn_free_wallet(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Wallet).Destroy)
	return result
}

func (c FfiConverterWallet) Read(reader io.Reader) *Wallet {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterWallet) Lower(value *Wallet) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Wallet")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterWallet) Write(writer io.Writer, value *Wallet) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerWallet struct{}

func (_ FfiDestroyerWallet) Destroy(value *Wallet) {
	value.Destroy()
}

type AssetCfa struct {
	AssetId      string
	Name         string
	Details      *string
	Precision    uint8
	IssuedSupply uint64
	Timestamp    int64
	AddedAt      int64
	Balance      Balance
	Media        *Media
}

func (r *AssetCfa) Destroy() {
	FfiDestroyerString{}.Destroy(r.AssetId)
	FfiDestroyerString{}.Destroy(r.Name)
	FfiDestroyerOptionalString{}.Destroy(r.Details)
	FfiDestroyerUint8{}.Destroy(r.Precision)
	FfiDestroyerUint64{}.Destroy(r.IssuedSupply)
	FfiDestroyerInt64{}.Destroy(r.Timestamp)
	FfiDestroyerInt64{}.Destroy(r.AddedAt)
	FfiDestroyerBalance{}.Destroy(r.Balance)
	FfiDestroyerOptionalMedia{}.Destroy(r.Media)
}

type FfiConverterAssetCfa struct{}

var FfiConverterAssetCfaINSTANCE = FfiConverterAssetCfa{}

func (c FfiConverterAssetCfa) Lift(rb RustBufferI) AssetCfa {
	return LiftFromRustBuffer[AssetCfa](c, rb)
}

func (c FfiConverterAssetCfa) Read(reader io.Reader) AssetCfa {
	return AssetCfa{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterUint8INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterInt64INSTANCE.Read(reader),
		FfiConverterInt64INSTANCE.Read(reader),
		FfiConverterBalanceINSTANCE.Read(reader),
		FfiConverterOptionalMediaINSTANCE.Read(reader),
	}
}

func (c FfiConverterAssetCfa) Lower(value AssetCfa) C.RustBuffer {
	return LowerIntoRustBuffer[AssetCfa](c, value)
}

func (c FfiConverterAssetCfa) Write(writer io.Writer, value AssetCfa) {
	FfiConverterStringINSTANCE.Write(writer, value.AssetId)
	FfiConverterStringINSTANCE.Write(writer, value.Name)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Details)
	FfiConverterUint8INSTANCE.Write(writer, value.Precision)
	FfiConverterUint64INSTANCE.Write(writer, value.IssuedSupply)
	FfiConverterInt64INSTANCE.Write(writer, value.Timestamp)
	FfiConverterInt64INSTANCE.Write(writer, value.AddedAt)
	FfiConverterBalanceINSTANCE.Write(writer, value.Balance)
	FfiConverterOptionalMediaINSTANCE.Write(writer, value.Media)
}

type FfiDestroyerAssetCfa struct{}

func (_ FfiDestroyerAssetCfa) Destroy(value AssetCfa) {
	value.Destroy()
}

type AssetIfa struct {
	AssetId      string
	Ticker       string
	Name         string
	Details      *string
	Precision    uint8
	IssuedSupply uint64
	Timestamp    int64
	AddedAt      int64
	Balance      Balance
	Media        *Media
}

func (r *AssetIfa) Destroy() {
	FfiDestroyerString{}.Destroy(r.AssetId)
	FfiDestroyerString{}.Destroy(r.Ticker)
	FfiDestroyerString{}.Destroy(r.Name)
	FfiDestroyerOptionalString{}.Destroy(r.Details)
	FfiDestroyerUint8{}.Destroy(r.Precision)
	FfiDestroyerUint64{}.Destroy(r.IssuedSupply)
	FfiDestroyerInt64{}.Destroy(r.Timestamp)
	FfiDestroyerInt64{}.Destroy(r.AddedAt)
	FfiDestroyerBalance{}.Destroy(r.Balance)
	FfiDestroyerOptionalMedia{}.Destroy(r.Media)
}

type FfiConverterAssetIfa struct{}

var FfiConverterAssetIfaINSTANCE = FfiConverterAssetIfa{}

func (c FfiConverterAssetIfa) Lift(rb RustBufferI) AssetIfa {
	return LiftFromRustBuffer[AssetIfa](c, rb)
}

func (c FfiConverterAssetIfa) Read(reader io.Reader) AssetIfa {
	return AssetIfa{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterUint8INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterInt64INSTANCE.Read(reader),
		FfiConverterInt64INSTANCE.Read(reader),
		FfiConverterBalanceINSTANCE.Read(reader),
		FfiConverterOptionalMediaINSTANCE.Read(reader),
	}
}

func (c FfiConverterAssetIfa) Lower(value AssetIfa) C.RustBuffer {
	return LowerIntoRustBuffer[AssetIfa](c, value)
}

func (c FfiConverterAssetIfa) Write(writer io.Writer, value AssetIfa) {
	FfiConverterStringINSTANCE.Write(writer, value.AssetId)
	FfiConverterStringINSTANCE.Write(writer, value.Ticker)
	FfiConverterStringINSTANCE.Write(writer, value.Name)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Details)
	FfiConverterUint8INSTANCE.Write(writer, value.Precision)
	FfiConverterUint64INSTANCE.Write(writer, value.IssuedSupply)
	FfiConverterInt64INSTANCE.Write(writer, value.Timestamp)
	FfiConverterInt64INSTANCE.Write(writer, value.AddedAt)
	FfiConverterBalanceINSTANCE.Write(writer, value.Balance)
	FfiConverterOptionalMediaINSTANCE.Write(writer, value.Media)
}

type FfiDestroyerAssetIfa struct{}

func (_ FfiDestroyerAssetIfa) Destroy(value AssetIfa) {
	value.Destroy()
}

type AssetNia struct {
	AssetId      string
	Ticker       string
	Name         string
	Details      *string
	Precision    uint8
	IssuedSupply uint64
	Timestamp    int64
	AddedAt      int64
	Balance      Balance
	Media        *Media
}

func (r *AssetNia) Destroy() {
	FfiDestroyerString{}.Destroy(r.AssetId)
	FfiDestroyerString{}.Destroy(r.Ticker)
	FfiDestroyerString{}.Destroy(r.Name)
	FfiDestroyerOptionalString{}.Destroy(r.Details)
	FfiDestroyerUint8{}.Destroy(r.Precision)
	FfiDestroyerUint64{}.Destroy(r.IssuedSupply)
	FfiDestroyerInt64{}.Destroy(r.Timestamp)
	FfiDestroyerInt64{}.Destroy(r.AddedAt)
	FfiDestroyerBalance{}.Destroy(r.Balance)
	FfiDestroyerOptionalMedia{}.Destroy(r.Media)
}

type FfiConverterAssetNia struct{}

var FfiConverterAssetNiaINSTANCE = FfiConverterAssetNia{}

func (c FfiConverterAssetNia) Lift(rb RustBufferI) AssetNia {
	return LiftFromRustBuffer[AssetNia](c, rb)
}

func (c FfiConverterAssetNia) Read(reader io.Reader) AssetNia {
	return AssetNia{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterUint8INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterInt64INSTANCE.Read(reader),
		FfiConverterInt64INSTANCE.Read(reader),
		FfiConverterBalanceINSTANCE.Read(reader),
		FfiConverterOptionalMediaINSTANCE.Read(reader),
	}
}

func (c FfiConverterAssetNia) Lower(value AssetNia) C.RustBuffer {
	return LowerIntoRustBuffer[AssetNia](c, value)
}

func (c FfiConverterAssetNia) Write(writer io.Writer, value AssetNia) {
	FfiConverterStringINSTANCE.Write(writer, value.AssetId)
	FfiConverterStringINSTANCE.Write(writer, value.Ticker)
	FfiConverterStringINSTANCE.Write(writer, value.Name)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Details)
	FfiConverterUint8INSTANCE.Write(writer, value.Precision)
	FfiConverterUint64INSTANCE.Write(writer, value.IssuedSupply)
	FfiConverterInt64INSTANCE.Write(writer, value.Timestamp)
	FfiConverterInt64INSTANCE.Write(writer, value.AddedAt)
	FfiConverterBalanceINSTANCE.Write(writer, value.Balance)
	FfiConverterOptionalMediaINSTANCE.Write(writer, value.Media)
}

type FfiDestroyerAssetNia struct{}

func (_ FfiDestroyerAssetNia) Destroy(value AssetNia) {
	value.Destroy()
}

type AssetUda struct {
	AssetId      string
	Ticker       string
	Name         string
	Details      *string
	Precision    uint8
	IssuedSupply uint64
	Timestamp    int64
	AddedAt      int64
	Balance      Balance
	Token        *TokenLight
}

func (r *AssetUda) Destroy() {
	FfiDestroyerString{}.Destroy(r.AssetId)
	FfiDestroyerString{}.Destroy(r.Ticker)
	FfiDestroyerString{}.Destroy(r.Name)
	FfiDestroyerOptionalString{}.Destroy(r.Details)
	FfiDestroyerUint8{}.Destroy(r.Precision)
	FfiDestroyerUint64{}.Destroy(r.IssuedSupply)
	FfiDestroyerInt64{}.Destroy(r.Timestamp)
	FfiDestroyerInt64{}.Destroy(r.AddedAt)
	FfiDestroyerBalance{}.Destroy(r.Balance)
	FfiDestroyerOptionalTokenLight{}.Destroy(r.Token)
}

type FfiConverterAssetUda struct{}

var FfiConverterAssetUdaINSTANCE = FfiConverterAssetUda{}

func (c FfiConverterAssetUda) Lift(rb RustBufferI) AssetUda {
	return LiftFromRustBuffer[AssetUda](c, rb)
}

func (c FfiConverterAssetUda) Read(reader io.Reader) AssetUda {
	return AssetUda{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterUint8INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterInt64INSTANCE.Read(reader),
		FfiConverterInt64INSTANCE.Read(reader),
		FfiConverterBalanceINSTANCE.Read(reader),
		FfiConverterOptionalTokenLightINSTANCE.Read(reader),
	}
}

func (c FfiConverterAssetUda) Lower(value AssetUda) C.RustBuffer {
	return LowerIntoRustBuffer[AssetUda](c, value)
}

func (c FfiConverterAssetUda) Write(writer io.Writer, value AssetUda) {
	FfiConverterStringINSTANCE.Write(writer, value.AssetId)
	FfiConverterStringINSTANCE.Write(writer, value.Ticker)
	FfiConverterStringINSTANCE.Write(writer, value.Name)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Details)
	FfiConverterUint8INSTANCE.Write(writer, value.Precision)
	FfiConverterUint64INSTANCE.Write(writer, value.IssuedSupply)
	FfiConverterInt64INSTANCE.Write(writer, value.Timestamp)
	FfiConverterInt64INSTANCE.Write(writer, value.AddedAt)
	FfiConverterBalanceINSTANCE.Write(writer, value.Balance)
	FfiConverterOptionalTokenLightINSTANCE.Write(writer, value.Token)
}

type FfiDestroyerAssetUda struct{}

func (_ FfiDestroyerAssetUda) Destroy(value AssetUda) {
	value.Destroy()
}

type Assets struct {
	Nia *[]AssetNia
	Uda *[]AssetUda
	Cfa *[]AssetCfa
	Ifa *[]AssetIfa
}

func (r *Assets) Destroy() {
	FfiDestroyerOptionalSequenceAssetNia{}.Destroy(r.Nia)
	FfiDestroyerOptionalSequenceAssetUda{}.Destroy(r.Uda)
	FfiDestroyerOptionalSequenceAssetCfa{}.Destroy(r.Cfa)
	FfiDestroyerOptionalSequenceAssetIfa{}.Destroy(r.Ifa)
}

type FfiConverterAssets struct{}

var FfiConverterAssetsINSTANCE = FfiConverterAssets{}

func (c FfiConverterAssets) Lift(rb RustBufferI) Assets {
	return LiftFromRustBuffer[Assets](c, rb)
}

func (c FfiConverterAssets) Read(reader io.Reader) Assets {
	return Assets{
		FfiConverterOptionalSequenceAssetNiaINSTANCE.Read(reader),
		FfiConverterOptionalSequenceAssetUdaINSTANCE.Read(reader),
		FfiConverterOptionalSequenceAssetCfaINSTANCE.Read(reader),
		FfiConverterOptionalSequenceAssetIfaINSTANCE.Read(reader),
	}
}

func (c FfiConverterAssets) Lower(value Assets) C.RustBuffer {
	return LowerIntoRustBuffer[Assets](c, value)
}

func (c FfiConverterAssets) Write(writer io.Writer, value Assets) {
	FfiConverterOptionalSequenceAssetNiaINSTANCE.Write(writer, value.Nia)
	FfiConverterOptionalSequenceAssetUdaINSTANCE.Write(writer, value.Uda)
	FfiConverterOptionalSequenceAssetCfaINSTANCE.Write(writer, value.Cfa)
	FfiConverterOptionalSequenceAssetIfaINSTANCE.Write(writer, value.Ifa)
}

type FfiDestroyerAssets struct{}

func (_ FfiDestroyerAssets) Destroy(value Assets) {
	value.Destroy()
}

type AssignmentsCollection struct {
	Fungible    uint64
	NonFungible bool
	Inflation   uint64
	Replace     uint8
}

func (r *AssignmentsCollection) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.Fungible)
	FfiDestroyerBool{}.Destroy(r.NonFungible)
	FfiDestroyerUint64{}.Destroy(r.Inflation)
	FfiDestroyerUint8{}.Destroy(r.Replace)
}

type FfiConverterAssignmentsCollection struct{}

var FfiConverterAssignmentsCollectionINSTANCE = FfiConverterAssignmentsCollection{}

func (c FfiConverterAssignmentsCollection) Lift(rb RustBufferI) AssignmentsCollection {
	return LiftFromRustBuffer[AssignmentsCollection](c, rb)
}

func (c FfiConverterAssignmentsCollection) Read(reader io.Reader) AssignmentsCollection {
	return AssignmentsCollection{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterBoolINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint8INSTANCE.Read(reader),
	}
}

func (c FfiConverterAssignmentsCollection) Lower(value AssignmentsCollection) C.RustBuffer {
	return LowerIntoRustBuffer[AssignmentsCollection](c, value)
}

func (c FfiConverterAssignmentsCollection) Write(writer io.Writer, value AssignmentsCollection) {
	FfiConverterUint64INSTANCE.Write(writer, value.Fungible)
	FfiConverterBoolINSTANCE.Write(writer, value.NonFungible)
	FfiConverterUint64INSTANCE.Write(writer, value.Inflation)
	FfiConverterUint8INSTANCE.Write(writer, value.Replace)
}

type FfiDestroyerAssignmentsCollection struct{}

func (_ FfiDestroyerAssignmentsCollection) Destroy(value AssignmentsCollection) {
	value.Destroy()
}

type Balance struct {
	Settled   uint64
	Future    uint64
	Spendable uint64
}

func (r *Balance) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.Settled)
	FfiDestroyerUint64{}.Destroy(r.Future)
	FfiDestroyerUint64{}.Destroy(r.Spendable)
}

type FfiConverterBalance struct{}

var FfiConverterBalanceINSTANCE = FfiConverterBalance{}

func (c FfiConverterBalance) Lift(rb RustBufferI) Balance {
	return LiftFromRustBuffer[Balance](c, rb)
}

func (c FfiConverterBalance) Read(reader io.Reader) Balance {
	return Balance{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterBalance) Lower(value Balance) C.RustBuffer {
	return LowerIntoRustBuffer[Balance](c, value)
}

func (c FfiConverterBalance) Write(writer io.Writer, value Balance) {
	FfiConverterUint64INSTANCE.Write(writer, value.Settled)
	FfiConverterUint64INSTANCE.Write(writer, value.Future)
	FfiConverterUint64INSTANCE.Write(writer, value.Spendable)
}

type FfiDestroyerBalance struct{}

func (_ FfiDestroyerBalance) Destroy(value Balance) {
	value.Destroy()
}

type BlockTime struct {
	Height    uint32
	Timestamp uint64
}

func (r *BlockTime) Destroy() {
	FfiDestroyerUint32{}.Destroy(r.Height)
	FfiDestroyerUint64{}.Destroy(r.Timestamp)
}

type FfiConverterBlockTime struct{}

var FfiConverterBlockTimeINSTANCE = FfiConverterBlockTime{}

func (c FfiConverterBlockTime) Lift(rb RustBufferI) BlockTime {
	return LiftFromRustBuffer[BlockTime](c, rb)
}

func (c FfiConverterBlockTime) Read(reader io.Reader) BlockTime {
	return BlockTime{
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterBlockTime) Lower(value BlockTime) C.RustBuffer {
	return LowerIntoRustBuffer[BlockTime](c, value)
}

func (c FfiConverterBlockTime) Write(writer io.Writer, value BlockTime) {
	FfiConverterUint32INSTANCE.Write(writer, value.Height)
	FfiConverterUint64INSTANCE.Write(writer, value.Timestamp)
}

type FfiDestroyerBlockTime struct{}

func (_ FfiDestroyerBlockTime) Destroy(value BlockTime) {
	value.Destroy()
}

type BtcBalance struct {
	Vanilla Balance
	Colored Balance
}

func (r *BtcBalance) Destroy() {
	FfiDestroyerBalance{}.Destroy(r.Vanilla)
	FfiDestroyerBalance{}.Destroy(r.Colored)
}

type FfiConverterBtcBalance struct{}

var FfiConverterBtcBalanceINSTANCE = FfiConverterBtcBalance{}

func (c FfiConverterBtcBalance) Lift(rb RustBufferI) BtcBalance {
	return LiftFromRustBuffer[BtcBalance](c, rb)
}

func (c FfiConverterBtcBalance) Read(reader io.Reader) BtcBalance {
	return BtcBalance{
		FfiConverterBalanceINSTANCE.Read(reader),
		FfiConverterBalanceINSTANCE.Read(reader),
	}
}

func (c FfiConverterBtcBalance) Lower(value BtcBalance) C.RustBuffer {
	return LowerIntoRustBuffer[BtcBalance](c, value)
}

func (c FfiConverterBtcBalance) Write(writer io.Writer, value BtcBalance) {
	FfiConverterBalanceINSTANCE.Write(writer, value.Vanilla)
	FfiConverterBalanceINSTANCE.Write(writer, value.Colored)
}

type FfiDestroyerBtcBalance struct{}

func (_ FfiDestroyerBtcBalance) Destroy(value BtcBalance) {
	value.Destroy()
}

type EmbeddedMedia struct {
	Mime string
	Data []uint8
}

func (r *EmbeddedMedia) Destroy() {
	FfiDestroyerString{}.Destroy(r.Mime)
	FfiDestroyerSequenceUint8{}.Destroy(r.Data)
}

type FfiConverterEmbeddedMedia struct{}

var FfiConverterEmbeddedMediaINSTANCE = FfiConverterEmbeddedMedia{}

func (c FfiConverterEmbeddedMedia) Lift(rb RustBufferI) EmbeddedMedia {
	return LiftFromRustBuffer[EmbeddedMedia](c, rb)
}

func (c FfiConverterEmbeddedMedia) Read(reader io.Reader) EmbeddedMedia {
	return EmbeddedMedia{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterSequenceUint8INSTANCE.Read(reader),
	}
}

func (c FfiConverterEmbeddedMedia) Lower(value EmbeddedMedia) C.RustBuffer {
	return LowerIntoRustBuffer[EmbeddedMedia](c, value)
}

func (c FfiConverterEmbeddedMedia) Write(writer io.Writer, value EmbeddedMedia) {
	FfiConverterStringINSTANCE.Write(writer, value.Mime)
	FfiConverterSequenceUint8INSTANCE.Write(writer, value.Data)
}

type FfiDestroyerEmbeddedMedia struct{}

func (_ FfiDestroyerEmbeddedMedia) Destroy(value EmbeddedMedia) {
	value.Destroy()
}

type InvoiceData struct {
	RecipientId         string
	AssetSchema         *AssetSchema
	AssetId             *string
	Assignment          Assignment
	AssignmentName      *string
	Network             BitcoinNetwork
	ExpirationTimestamp *int64
	TransportEndpoints  []string
}

func (r *InvoiceData) Destroy() {
	FfiDestroyerString{}.Destroy(r.RecipientId)
	FfiDestroyerOptionalAssetSchema{}.Destroy(r.AssetSchema)
	FfiDestroyerOptionalString{}.Destroy(r.AssetId)
	FfiDestroyerAssignment{}.Destroy(r.Assignment)
	FfiDestroyerOptionalString{}.Destroy(r.AssignmentName)
	FfiDestroyerBitcoinNetwork{}.Destroy(r.Network)
	FfiDestroyerOptionalInt64{}.Destroy(r.ExpirationTimestamp)
	FfiDestroyerSequenceString{}.Destroy(r.TransportEndpoints)
}

type FfiConverterInvoiceData struct{}

var FfiConverterInvoiceDataINSTANCE = FfiConverterInvoiceData{}

func (c FfiConverterInvoiceData) Lift(rb RustBufferI) InvoiceData {
	return LiftFromRustBuffer[InvoiceData](c, rb)
}

func (c FfiConverterInvoiceData) Read(reader io.Reader) InvoiceData {
	return InvoiceData{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterOptionalAssetSchemaINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterAssignmentINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterBitcoinNetworkINSTANCE.Read(reader),
		FfiConverterOptionalInt64INSTANCE.Read(reader),
		FfiConverterSequenceStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterInvoiceData) Lower(value InvoiceData) C.RustBuffer {
	return LowerIntoRustBuffer[InvoiceData](c, value)
}

func (c FfiConverterInvoiceData) Write(writer io.Writer, value InvoiceData) {
	FfiConverterStringINSTANCE.Write(writer, value.RecipientId)
	FfiConverterOptionalAssetSchemaINSTANCE.Write(writer, value.AssetSchema)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.AssetId)
	FfiConverterAssignmentINSTANCE.Write(writer, value.Assignment)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.AssignmentName)
	FfiConverterBitcoinNetworkINSTANCE.Write(writer, value.Network)
	FfiConverterOptionalInt64INSTANCE.Write(writer, value.ExpirationTimestamp)
	FfiConverterSequenceStringINSTANCE.Write(writer, value.TransportEndpoints)
}

type FfiDestroyerInvoiceData struct{}

func (_ FfiDestroyerInvoiceData) Destroy(value InvoiceData) {
	value.Destroy()
}

type Keys struct {
	Mnemonic           string
	Xpub               string
	AccountXpubVanilla string
	AccountXpubColored string
	MasterFingerprint  string
}

func (r *Keys) Destroy() {
	FfiDestroyerString{}.Destroy(r.Mnemonic)
	FfiDestroyerString{}.Destroy(r.Xpub)
	FfiDestroyerString{}.Destroy(r.AccountXpubVanilla)
	FfiDestroyerString{}.Destroy(r.AccountXpubColored)
	FfiDestroyerString{}.Destroy(r.MasterFingerprint)
}

type FfiConverterKeys struct{}

var FfiConverterKeysINSTANCE = FfiConverterKeys{}

func (c FfiConverterKeys) Lift(rb RustBufferI) Keys {
	return LiftFromRustBuffer[Keys](c, rb)
}

func (c FfiConverterKeys) Read(reader io.Reader) Keys {
	return Keys{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterKeys) Lower(value Keys) C.RustBuffer {
	return LowerIntoRustBuffer[Keys](c, value)
}

func (c FfiConverterKeys) Write(writer io.Writer, value Keys) {
	FfiConverterStringINSTANCE.Write(writer, value.Mnemonic)
	FfiConverterStringINSTANCE.Write(writer, value.Xpub)
	FfiConverterStringINSTANCE.Write(writer, value.AccountXpubVanilla)
	FfiConverterStringINSTANCE.Write(writer, value.AccountXpubColored)
	FfiConverterStringINSTANCE.Write(writer, value.MasterFingerprint)
}

type FfiDestroyerKeys struct{}

func (_ FfiDestroyerKeys) Destroy(value Keys) {
	value.Destroy()
}

type Media struct {
	FilePath string
	Digest   string
	Mime     string
}

func (r *Media) Destroy() {
	FfiDestroyerString{}.Destroy(r.FilePath)
	FfiDestroyerString{}.Destroy(r.Digest)
	FfiDestroyerString{}.Destroy(r.Mime)
}

type FfiConverterMedia struct{}

var FfiConverterMediaINSTANCE = FfiConverterMedia{}

func (c FfiConverterMedia) Lift(rb RustBufferI) Media {
	return LiftFromRustBuffer[Media](c, rb)
}

func (c FfiConverterMedia) Read(reader io.Reader) Media {
	return Media{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterMedia) Lower(value Media) C.RustBuffer {
	return LowerIntoRustBuffer[Media](c, value)
}

func (c FfiConverterMedia) Write(writer io.Writer, value Media) {
	FfiConverterStringINSTANCE.Write(writer, value.FilePath)
	FfiConverterStringINSTANCE.Write(writer, value.Digest)
	FfiConverterStringINSTANCE.Write(writer, value.Mime)
}

type FfiDestroyerMedia struct{}

func (_ FfiDestroyerMedia) Destroy(value Media) {
	value.Destroy()
}

type Metadata struct {
	AssetSchema  AssetSchema
	IssuedSupply uint64
	Timestamp    int64
	Name         string
	Precision    uint8
	Ticker       *string
	Details      *string
	Token        *Token
}

func (r *Metadata) Destroy() {
	FfiDestroyerAssetSchema{}.Destroy(r.AssetSchema)
	FfiDestroyerUint64{}.Destroy(r.IssuedSupply)
	FfiDestroyerInt64{}.Destroy(r.Timestamp)
	FfiDestroyerString{}.Destroy(r.Name)
	FfiDestroyerUint8{}.Destroy(r.Precision)
	FfiDestroyerOptionalString{}.Destroy(r.Ticker)
	FfiDestroyerOptionalString{}.Destroy(r.Details)
	FfiDestroyerOptionalToken{}.Destroy(r.Token)
}

type FfiConverterMetadata struct{}

var FfiConverterMetadataINSTANCE = FfiConverterMetadata{}

func (c FfiConverterMetadata) Lift(rb RustBufferI) Metadata {
	return LiftFromRustBuffer[Metadata](c, rb)
}

func (c FfiConverterMetadata) Read(reader io.Reader) Metadata {
	return Metadata{
		FfiConverterAssetSchemaINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterInt64INSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterUint8INSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalTokenINSTANCE.Read(reader),
	}
}

func (c FfiConverterMetadata) Lower(value Metadata) C.RustBuffer {
	return LowerIntoRustBuffer[Metadata](c, value)
}

func (c FfiConverterMetadata) Write(writer io.Writer, value Metadata) {
	FfiConverterAssetSchemaINSTANCE.Write(writer, value.AssetSchema)
	FfiConverterUint64INSTANCE.Write(writer, value.IssuedSupply)
	FfiConverterInt64INSTANCE.Write(writer, value.Timestamp)
	FfiConverterStringINSTANCE.Write(writer, value.Name)
	FfiConverterUint8INSTANCE.Write(writer, value.Precision)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Ticker)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Details)
	FfiConverterOptionalTokenINSTANCE.Write(writer, value.Token)
}

type FfiDestroyerMetadata struct{}

func (_ FfiDestroyerMetadata) Destroy(value Metadata) {
	value.Destroy()
}

type Online struct {
	Id         uint64
	IndexerUrl string
}

func (r *Online) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.Id)
	FfiDestroyerString{}.Destroy(r.IndexerUrl)
}

type FfiConverterOnline struct{}

var FfiConverterOnlineINSTANCE = FfiConverterOnline{}

func (c FfiConverterOnline) Lift(rb RustBufferI) Online {
	return LiftFromRustBuffer[Online](c, rb)
}

func (c FfiConverterOnline) Read(reader io.Reader) Online {
	return Online{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterOnline) Lower(value Online) C.RustBuffer {
	return LowerIntoRustBuffer[Online](c, value)
}

func (c FfiConverterOnline) Write(writer io.Writer, value Online) {
	FfiConverterUint64INSTANCE.Write(writer, value.Id)
	FfiConverterStringINSTANCE.Write(writer, value.IndexerUrl)
}

type FfiDestroyerOnline struct{}

func (_ FfiDestroyerOnline) Destroy(value Online) {
	value.Destroy()
}

type Outpoint struct {
	Txid string
	Vout uint32
}

func (r *Outpoint) Destroy() {
	FfiDestroyerString{}.Destroy(r.Txid)
	FfiDestroyerUint32{}.Destroy(r.Vout)
}

type FfiConverterOutpoint struct{}

var FfiConverterOutpointINSTANCE = FfiConverterOutpoint{}

func (c FfiConverterOutpoint) Lift(rb RustBufferI) Outpoint {
	return LiftFromRustBuffer[Outpoint](c, rb)
}

func (c FfiConverterOutpoint) Read(reader io.Reader) Outpoint {
	return Outpoint{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterUint32INSTANCE.Read(reader),
	}
}

func (c FfiConverterOutpoint) Lower(value Outpoint) C.RustBuffer {
	return LowerIntoRustBuffer[Outpoint](c, value)
}

func (c FfiConverterOutpoint) Write(writer io.Writer, value Outpoint) {
	FfiConverterStringINSTANCE.Write(writer, value.Txid)
	FfiConverterUint32INSTANCE.Write(writer, value.Vout)
}

type FfiDestroyerOutpoint struct{}

func (_ FfiDestroyerOutpoint) Destroy(value Outpoint) {
	value.Destroy()
}

type ProofOfReserves struct {
	Utxo  Outpoint
	Proof []uint8
}

func (r *ProofOfReserves) Destroy() {
	FfiDestroyerOutpoint{}.Destroy(r.Utxo)
	FfiDestroyerSequenceUint8{}.Destroy(r.Proof)
}

type FfiConverterProofOfReserves struct{}

var FfiConverterProofOfReservesINSTANCE = FfiConverterProofOfReserves{}

func (c FfiConverterProofOfReserves) Lift(rb RustBufferI) ProofOfReserves {
	return LiftFromRustBuffer[ProofOfReserves](c, rb)
}

func (c FfiConverterProofOfReserves) Read(reader io.Reader) ProofOfReserves {
	return ProofOfReserves{
		FfiConverterOutpointINSTANCE.Read(reader),
		FfiConverterSequenceUint8INSTANCE.Read(reader),
	}
}

func (c FfiConverterProofOfReserves) Lower(value ProofOfReserves) C.RustBuffer {
	return LowerIntoRustBuffer[ProofOfReserves](c, value)
}

func (c FfiConverterProofOfReserves) Write(writer io.Writer, value ProofOfReserves) {
	FfiConverterOutpointINSTANCE.Write(writer, value.Utxo)
	FfiConverterSequenceUint8INSTANCE.Write(writer, value.Proof)
}

type FfiDestroyerProofOfReserves struct{}

func (_ FfiDestroyerProofOfReserves) Destroy(value ProofOfReserves) {
	value.Destroy()
}

type ReceiveData struct {
	Invoice             string
	RecipientId         string
	ExpirationTimestamp *int64
	BatchTransferIdx    int32
}

func (r *ReceiveData) Destroy() {
	FfiDestroyerString{}.Destroy(r.Invoice)
	FfiDestroyerString{}.Destroy(r.RecipientId)
	FfiDestroyerOptionalInt64{}.Destroy(r.ExpirationTimestamp)
	FfiDestroyerInt32{}.Destroy(r.BatchTransferIdx)
}

type FfiConverterReceiveData struct{}

var FfiConverterReceiveDataINSTANCE = FfiConverterReceiveData{}

func (c FfiConverterReceiveData) Lift(rb RustBufferI) ReceiveData {
	return LiftFromRustBuffer[ReceiveData](c, rb)
}

func (c FfiConverterReceiveData) Read(reader io.Reader) ReceiveData {
	return ReceiveData{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterOptionalInt64INSTANCE.Read(reader),
		FfiConverterInt32INSTANCE.Read(reader),
	}
}

func (c FfiConverterReceiveData) Lower(value ReceiveData) C.RustBuffer {
	return LowerIntoRustBuffer[ReceiveData](c, value)
}

func (c FfiConverterReceiveData) Write(writer io.Writer, value ReceiveData) {
	FfiConverterStringINSTANCE.Write(writer, value.Invoice)
	FfiConverterStringINSTANCE.Write(writer, value.RecipientId)
	FfiConverterOptionalInt64INSTANCE.Write(writer, value.ExpirationTimestamp)
	FfiConverterInt32INSTANCE.Write(writer, value.BatchTransferIdx)
}

type FfiDestroyerReceiveData struct{}

func (_ FfiDestroyerReceiveData) Destroy(value ReceiveData) {
	value.Destroy()
}

type Recipient struct {
	RecipientId        string
	WitnessData        *WitnessData
	Assignment         Assignment
	TransportEndpoints []string
}

func (r *Recipient) Destroy() {
	FfiDestroyerString{}.Destroy(r.RecipientId)
	FfiDestroyerOptionalWitnessData{}.Destroy(r.WitnessData)
	FfiDestroyerAssignment{}.Destroy(r.Assignment)
	FfiDestroyerSequenceString{}.Destroy(r.TransportEndpoints)
}

type FfiConverterRecipient struct{}

var FfiConverterRecipientINSTANCE = FfiConverterRecipient{}

func (c FfiConverterRecipient) Lift(rb RustBufferI) Recipient {
	return LiftFromRustBuffer[Recipient](c, rb)
}

func (c FfiConverterRecipient) Read(reader io.Reader) Recipient {
	return Recipient{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterOptionalWitnessDataINSTANCE.Read(reader),
		FfiConverterAssignmentINSTANCE.Read(reader),
		FfiConverterSequenceStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterRecipient) Lower(value Recipient) C.RustBuffer {
	return LowerIntoRustBuffer[Recipient](c, value)
}

func (c FfiConverterRecipient) Write(writer io.Writer, value Recipient) {
	FfiConverterStringINSTANCE.Write(writer, value.RecipientId)
	FfiConverterOptionalWitnessDataINSTANCE.Write(writer, value.WitnessData)
	FfiConverterAssignmentINSTANCE.Write(writer, value.Assignment)
	FfiConverterSequenceStringINSTANCE.Write(writer, value.TransportEndpoints)
}

type FfiDestroyerRecipient struct{}

func (_ FfiDestroyerRecipient) Destroy(value Recipient) {
	value.Destroy()
}

type RefreshFilter struct {
	Status   RefreshTransferStatus
	Incoming bool
}

func (r *RefreshFilter) Destroy() {
	FfiDestroyerRefreshTransferStatus{}.Destroy(r.Status)
	FfiDestroyerBool{}.Destroy(r.Incoming)
}

type FfiConverterRefreshFilter struct{}

var FfiConverterRefreshFilterINSTANCE = FfiConverterRefreshFilter{}

func (c FfiConverterRefreshFilter) Lift(rb RustBufferI) RefreshFilter {
	return LiftFromRustBuffer[RefreshFilter](c, rb)
}

func (c FfiConverterRefreshFilter) Read(reader io.Reader) RefreshFilter {
	return RefreshFilter{
		FfiConverterRefreshTransferStatusINSTANCE.Read(reader),
		FfiConverterBoolINSTANCE.Read(reader),
	}
}

func (c FfiConverterRefreshFilter) Lower(value RefreshFilter) C.RustBuffer {
	return LowerIntoRustBuffer[RefreshFilter](c, value)
}

func (c FfiConverterRefreshFilter) Write(writer io.Writer, value RefreshFilter) {
	FfiConverterRefreshTransferStatusINSTANCE.Write(writer, value.Status)
	FfiConverterBoolINSTANCE.Write(writer, value.Incoming)
}

type FfiDestroyerRefreshFilter struct{}

func (_ FfiDestroyerRefreshFilter) Destroy(value RefreshFilter) {
	value.Destroy()
}

type RefreshedTransfer struct {
	UpdatedStatus *TransferStatus
	Failure       **RgbLibError
}

func (r *RefreshedTransfer) Destroy() {
	FfiDestroyerOptionalTransferStatus{}.Destroy(r.UpdatedStatus)
	FfiDestroyerOptionalRgbLibError{}.Destroy(r.Failure)
}

type FfiConverterRefreshedTransfer struct{}

var FfiConverterRefreshedTransferINSTANCE = FfiConverterRefreshedTransfer{}

func (c FfiConverterRefreshedTransfer) Lift(rb RustBufferI) RefreshedTransfer {
	return LiftFromRustBuffer[RefreshedTransfer](c, rb)
}

func (c FfiConverterRefreshedTransfer) Read(reader io.Reader) RefreshedTransfer {
	return RefreshedTransfer{
		FfiConverterOptionalTransferStatusINSTANCE.Read(reader),
		FfiConverterOptionalRgbLibErrorINSTANCE.Read(reader),
	}
}

func (c FfiConverterRefreshedTransfer) Lower(value RefreshedTransfer) C.RustBuffer {
	return LowerIntoRustBuffer[RefreshedTransfer](c, value)
}

func (c FfiConverterRefreshedTransfer) Write(writer io.Writer, value RefreshedTransfer) {
	FfiConverterOptionalTransferStatusINSTANCE.Write(writer, value.UpdatedStatus)
	FfiConverterOptionalRgbLibErrorINSTANCE.Write(writer, value.Failure)
}

type FfiDestroyerRefreshedTransfer struct{}

func (_ FfiDestroyerRefreshedTransfer) Destroy(value RefreshedTransfer) {
	value.Destroy()
}

type RgbAllocation struct {
	AssetId    *string
	Assignment Assignment
	Settled    bool
}

func (r *RgbAllocation) Destroy() {
	FfiDestroyerOptionalString{}.Destroy(r.AssetId)
	FfiDestroyerAssignment{}.Destroy(r.Assignment)
	FfiDestroyerBool{}.Destroy(r.Settled)
}

type FfiConverterRgbAllocation struct{}

var FfiConverterRgbAllocationINSTANCE = FfiConverterRgbAllocation{}

func (c FfiConverterRgbAllocation) Lift(rb RustBufferI) RgbAllocation {
	return LiftFromRustBuffer[RgbAllocation](c, rb)
}

func (c FfiConverterRgbAllocation) Read(reader io.Reader) RgbAllocation {
	return RgbAllocation{
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterAssignmentINSTANCE.Read(reader),
		FfiConverterBoolINSTANCE.Read(reader),
	}
}

func (c FfiConverterRgbAllocation) Lower(value RgbAllocation) C.RustBuffer {
	return LowerIntoRustBuffer[RgbAllocation](c, value)
}

func (c FfiConverterRgbAllocation) Write(writer io.Writer, value RgbAllocation) {
	FfiConverterOptionalStringINSTANCE.Write(writer, value.AssetId)
	FfiConverterAssignmentINSTANCE.Write(writer, value.Assignment)
	FfiConverterBoolINSTANCE.Write(writer, value.Settled)
}

type FfiDestroyerRgbAllocation struct{}

func (_ FfiDestroyerRgbAllocation) Destroy(value RgbAllocation) {
	value.Destroy()
}

type SendResult struct {
	Txid             string
	BatchTransferIdx int32
}

func (r *SendResult) Destroy() {
	FfiDestroyerString{}.Destroy(r.Txid)
	FfiDestroyerInt32{}.Destroy(r.BatchTransferIdx)
}

type FfiConverterSendResult struct{}

var FfiConverterSendResultINSTANCE = FfiConverterSendResult{}

func (c FfiConverterSendResult) Lift(rb RustBufferI) SendResult {
	return LiftFromRustBuffer[SendResult](c, rb)
}

func (c FfiConverterSendResult) Read(reader io.Reader) SendResult {
	return SendResult{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterInt32INSTANCE.Read(reader),
	}
}

func (c FfiConverterSendResult) Lower(value SendResult) C.RustBuffer {
	return LowerIntoRustBuffer[SendResult](c, value)
}

func (c FfiConverterSendResult) Write(writer io.Writer, value SendResult) {
	FfiConverterStringINSTANCE.Write(writer, value.Txid)
	FfiConverterInt32INSTANCE.Write(writer, value.BatchTransferIdx)
}

type FfiDestroyerSendResult struct{}

func (_ FfiDestroyerSendResult) Destroy(value SendResult) {
	value.Destroy()
}

type Token struct {
	Index         uint32
	Ticker        *string
	Name          *string
	Details       *string
	EmbeddedMedia *EmbeddedMedia
	Media         *Media
	Attachments   map[uint8]Media
	Reserves      *ProofOfReserves
}

func (r *Token) Destroy() {
	FfiDestroyerUint32{}.Destroy(r.Index)
	FfiDestroyerOptionalString{}.Destroy(r.Ticker)
	FfiDestroyerOptionalString{}.Destroy(r.Name)
	FfiDestroyerOptionalString{}.Destroy(r.Details)
	FfiDestroyerOptionalEmbeddedMedia{}.Destroy(r.EmbeddedMedia)
	FfiDestroyerOptionalMedia{}.Destroy(r.Media)
	FfiDestroyerMapUint8Media{}.Destroy(r.Attachments)
	FfiDestroyerOptionalProofOfReserves{}.Destroy(r.Reserves)
}

type FfiConverterToken struct{}

var FfiConverterTokenINSTANCE = FfiConverterToken{}

func (c FfiConverterToken) Lift(rb RustBufferI) Token {
	return LiftFromRustBuffer[Token](c, rb)
}

func (c FfiConverterToken) Read(reader io.Reader) Token {
	return Token{
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalEmbeddedMediaINSTANCE.Read(reader),
		FfiConverterOptionalMediaINSTANCE.Read(reader),
		FfiConverterMapUint8MediaINSTANCE.Read(reader),
		FfiConverterOptionalProofOfReservesINSTANCE.Read(reader),
	}
}

func (c FfiConverterToken) Lower(value Token) C.RustBuffer {
	return LowerIntoRustBuffer[Token](c, value)
}

func (c FfiConverterToken) Write(writer io.Writer, value Token) {
	FfiConverterUint32INSTANCE.Write(writer, value.Index)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Ticker)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Name)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Details)
	FfiConverterOptionalEmbeddedMediaINSTANCE.Write(writer, value.EmbeddedMedia)
	FfiConverterOptionalMediaINSTANCE.Write(writer, value.Media)
	FfiConverterMapUint8MediaINSTANCE.Write(writer, value.Attachments)
	FfiConverterOptionalProofOfReservesINSTANCE.Write(writer, value.Reserves)
}

type FfiDestroyerToken struct{}

func (_ FfiDestroyerToken) Destroy(value Token) {
	value.Destroy()
}

type TokenLight struct {
	Index         uint32
	Ticker        *string
	Name          *string
	Details       *string
	EmbeddedMedia bool
	Media         *Media
	Attachments   map[uint8]Media
	Reserves      bool
}

func (r *TokenLight) Destroy() {
	FfiDestroyerUint32{}.Destroy(r.Index)
	FfiDestroyerOptionalString{}.Destroy(r.Ticker)
	FfiDestroyerOptionalString{}.Destroy(r.Name)
	FfiDestroyerOptionalString{}.Destroy(r.Details)
	FfiDestroyerBool{}.Destroy(r.EmbeddedMedia)
	FfiDestroyerOptionalMedia{}.Destroy(r.Media)
	FfiDestroyerMapUint8Media{}.Destroy(r.Attachments)
	FfiDestroyerBool{}.Destroy(r.Reserves)
}

type FfiConverterTokenLight struct{}

var FfiConverterTokenLightINSTANCE = FfiConverterTokenLight{}

func (c FfiConverterTokenLight) Lift(rb RustBufferI) TokenLight {
	return LiftFromRustBuffer[TokenLight](c, rb)
}

func (c FfiConverterTokenLight) Read(reader io.Reader) TokenLight {
	return TokenLight{
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterBoolINSTANCE.Read(reader),
		FfiConverterOptionalMediaINSTANCE.Read(reader),
		FfiConverterMapUint8MediaINSTANCE.Read(reader),
		FfiConverterBoolINSTANCE.Read(reader),
	}
}

func (c FfiConverterTokenLight) Lower(value TokenLight) C.RustBuffer {
	return LowerIntoRustBuffer[TokenLight](c, value)
}

func (c FfiConverterTokenLight) Write(writer io.Writer, value TokenLight) {
	FfiConverterUint32INSTANCE.Write(writer, value.Index)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Ticker)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Name)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Details)
	FfiConverterBoolINSTANCE.Write(writer, value.EmbeddedMedia)
	FfiConverterOptionalMediaINSTANCE.Write(writer, value.Media)
	FfiConverterMapUint8MediaINSTANCE.Write(writer, value.Attachments)
	FfiConverterBoolINSTANCE.Write(writer, value.Reserves)
}

type FfiDestroyerTokenLight struct{}

func (_ FfiDestroyerTokenLight) Destroy(value TokenLight) {
	value.Destroy()
}

type Transaction struct {
	TransactionType  TransactionType
	Txid             string
	Received         uint64
	Sent             uint64
	Fee              uint64
	ConfirmationTime *BlockTime
}

func (r *Transaction) Destroy() {
	FfiDestroyerTransactionType{}.Destroy(r.TransactionType)
	FfiDestroyerString{}.Destroy(r.Txid)
	FfiDestroyerUint64{}.Destroy(r.Received)
	FfiDestroyerUint64{}.Destroy(r.Sent)
	FfiDestroyerUint64{}.Destroy(r.Fee)
	FfiDestroyerOptionalBlockTime{}.Destroy(r.ConfirmationTime)
}

type FfiConverterTransaction struct{}

var FfiConverterTransactionINSTANCE = FfiConverterTransaction{}

func (c FfiConverterTransaction) Lift(rb RustBufferI) Transaction {
	return LiftFromRustBuffer[Transaction](c, rb)
}

func (c FfiConverterTransaction) Read(reader io.Reader) Transaction {
	return Transaction{
		FfiConverterTransactionTypeINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterOptionalBlockTimeINSTANCE.Read(reader),
	}
}

func (c FfiConverterTransaction) Lower(value Transaction) C.RustBuffer {
	return LowerIntoRustBuffer[Transaction](c, value)
}

func (c FfiConverterTransaction) Write(writer io.Writer, value Transaction) {
	FfiConverterTransactionTypeINSTANCE.Write(writer, value.TransactionType)
	FfiConverterStringINSTANCE.Write(writer, value.Txid)
	FfiConverterUint64INSTANCE.Write(writer, value.Received)
	FfiConverterUint64INSTANCE.Write(writer, value.Sent)
	FfiConverterUint64INSTANCE.Write(writer, value.Fee)
	FfiConverterOptionalBlockTimeINSTANCE.Write(writer, value.ConfirmationTime)
}

type FfiDestroyerTransaction struct{}

func (_ FfiDestroyerTransaction) Destroy(value Transaction) {
	value.Destroy()
}

type Transfer struct {
	Idx                 int32
	BatchTransferIdx    int32
	CreatedAt           int64
	UpdatedAt           int64
	Status              TransferStatus
	RequestedAssignment *Assignment
	Assignments         []Assignment
	Kind                TransferKind
	Txid                *string
	RecipientId         *string
	ReceiveUtxo         *Outpoint
	ChangeUtxo          *Outpoint
	Expiration          *int64
	TransportEndpoints  []TransferTransportEndpoint
	InvoiceString       *string
}

func (r *Transfer) Destroy() {
	FfiDestroyerInt32{}.Destroy(r.Idx)
	FfiDestroyerInt32{}.Destroy(r.BatchTransferIdx)
	FfiDestroyerInt64{}.Destroy(r.CreatedAt)
	FfiDestroyerInt64{}.Destroy(r.UpdatedAt)
	FfiDestroyerTransferStatus{}.Destroy(r.Status)
	FfiDestroyerOptionalAssignment{}.Destroy(r.RequestedAssignment)
	FfiDestroyerSequenceAssignment{}.Destroy(r.Assignments)
	FfiDestroyerTransferKind{}.Destroy(r.Kind)
	FfiDestroyerOptionalString{}.Destroy(r.Txid)
	FfiDestroyerOptionalString{}.Destroy(r.RecipientId)
	FfiDestroyerOptionalOutpoint{}.Destroy(r.ReceiveUtxo)
	FfiDestroyerOptionalOutpoint{}.Destroy(r.ChangeUtxo)
	FfiDestroyerOptionalInt64{}.Destroy(r.Expiration)
	FfiDestroyerSequenceTransferTransportEndpoint{}.Destroy(r.TransportEndpoints)
	FfiDestroyerOptionalString{}.Destroy(r.InvoiceString)
}

type FfiConverterTransfer struct{}

var FfiConverterTransferINSTANCE = FfiConverterTransfer{}

func (c FfiConverterTransfer) Lift(rb RustBufferI) Transfer {
	return LiftFromRustBuffer[Transfer](c, rb)
}

func (c FfiConverterTransfer) Read(reader io.Reader) Transfer {
	return Transfer{
		FfiConverterInt32INSTANCE.Read(reader),
		FfiConverterInt32INSTANCE.Read(reader),
		FfiConverterInt64INSTANCE.Read(reader),
		FfiConverterInt64INSTANCE.Read(reader),
		FfiConverterTransferStatusINSTANCE.Read(reader),
		FfiConverterOptionalAssignmentINSTANCE.Read(reader),
		FfiConverterSequenceAssignmentINSTANCE.Read(reader),
		FfiConverterTransferKindINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalOutpointINSTANCE.Read(reader),
		FfiConverterOptionalOutpointINSTANCE.Read(reader),
		FfiConverterOptionalInt64INSTANCE.Read(reader),
		FfiConverterSequenceTransferTransportEndpointINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterTransfer) Lower(value Transfer) C.RustBuffer {
	return LowerIntoRustBuffer[Transfer](c, value)
}

func (c FfiConverterTransfer) Write(writer io.Writer, value Transfer) {
	FfiConverterInt32INSTANCE.Write(writer, value.Idx)
	FfiConverterInt32INSTANCE.Write(writer, value.BatchTransferIdx)
	FfiConverterInt64INSTANCE.Write(writer, value.CreatedAt)
	FfiConverterInt64INSTANCE.Write(writer, value.UpdatedAt)
	FfiConverterTransferStatusINSTANCE.Write(writer, value.Status)
	FfiConverterOptionalAssignmentINSTANCE.Write(writer, value.RequestedAssignment)
	FfiConverterSequenceAssignmentINSTANCE.Write(writer, value.Assignments)
	FfiConverterTransferKindINSTANCE.Write(writer, value.Kind)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Txid)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.RecipientId)
	FfiConverterOptionalOutpointINSTANCE.Write(writer, value.ReceiveUtxo)
	FfiConverterOptionalOutpointINSTANCE.Write(writer, value.ChangeUtxo)
	FfiConverterOptionalInt64INSTANCE.Write(writer, value.Expiration)
	FfiConverterSequenceTransferTransportEndpointINSTANCE.Write(writer, value.TransportEndpoints)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.InvoiceString)
}

type FfiDestroyerTransfer struct{}

func (_ FfiDestroyerTransfer) Destroy(value Transfer) {
	value.Destroy()
}

type TransferTransportEndpoint struct {
	Endpoint      string
	TransportType TransportType
	Used          bool
}

func (r *TransferTransportEndpoint) Destroy() {
	FfiDestroyerString{}.Destroy(r.Endpoint)
	FfiDestroyerTransportType{}.Destroy(r.TransportType)
	FfiDestroyerBool{}.Destroy(r.Used)
}

type FfiConverterTransferTransportEndpoint struct{}

var FfiConverterTransferTransportEndpointINSTANCE = FfiConverterTransferTransportEndpoint{}

func (c FfiConverterTransferTransportEndpoint) Lift(rb RustBufferI) TransferTransportEndpoint {
	return LiftFromRustBuffer[TransferTransportEndpoint](c, rb)
}

func (c FfiConverterTransferTransportEndpoint) Read(reader io.Reader) TransferTransportEndpoint {
	return TransferTransportEndpoint{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterTransportTypeINSTANCE.Read(reader),
		FfiConverterBoolINSTANCE.Read(reader),
	}
}

func (c FfiConverterTransferTransportEndpoint) Lower(value TransferTransportEndpoint) C.RustBuffer {
	return LowerIntoRustBuffer[TransferTransportEndpoint](c, value)
}

func (c FfiConverterTransferTransportEndpoint) Write(writer io.Writer, value TransferTransportEndpoint) {
	FfiConverterStringINSTANCE.Write(writer, value.Endpoint)
	FfiConverterTransportTypeINSTANCE.Write(writer, value.TransportType)
	FfiConverterBoolINSTANCE.Write(writer, value.Used)
}

type FfiDestroyerTransferTransportEndpoint struct{}

func (_ FfiDestroyerTransferTransportEndpoint) Destroy(value TransferTransportEndpoint) {
	value.Destroy()
}

type Unspent struct {
	Utxo           Utxo
	RgbAllocations []RgbAllocation
	PendingBlinded uint32
}

func (r *Unspent) Destroy() {
	FfiDestroyerUtxo{}.Destroy(r.Utxo)
	FfiDestroyerSequenceRgbAllocation{}.Destroy(r.RgbAllocations)
	FfiDestroyerUint32{}.Destroy(r.PendingBlinded)
}

type FfiConverterUnspent struct{}

var FfiConverterUnspentINSTANCE = FfiConverterUnspent{}

func (c FfiConverterUnspent) Lift(rb RustBufferI) Unspent {
	return LiftFromRustBuffer[Unspent](c, rb)
}

func (c FfiConverterUnspent) Read(reader io.Reader) Unspent {
	return Unspent{
		FfiConverterUtxoINSTANCE.Read(reader),
		FfiConverterSequenceRgbAllocationINSTANCE.Read(reader),
		FfiConverterUint32INSTANCE.Read(reader),
	}
}

func (c FfiConverterUnspent) Lower(value Unspent) C.RustBuffer {
	return LowerIntoRustBuffer[Unspent](c, value)
}

func (c FfiConverterUnspent) Write(writer io.Writer, value Unspent) {
	FfiConverterUtxoINSTANCE.Write(writer, value.Utxo)
	FfiConverterSequenceRgbAllocationINSTANCE.Write(writer, value.RgbAllocations)
	FfiConverterUint32INSTANCE.Write(writer, value.PendingBlinded)
}

type FfiDestroyerUnspent struct{}

func (_ FfiDestroyerUnspent) Destroy(value Unspent) {
	value.Destroy()
}

type Utxo struct {
	Outpoint  Outpoint
	BtcAmount uint64
	Colorable bool
	Exists    bool
}

func (r *Utxo) Destroy() {
	FfiDestroyerOutpoint{}.Destroy(r.Outpoint)
	FfiDestroyerUint64{}.Destroy(r.BtcAmount)
	FfiDestroyerBool{}.Destroy(r.Colorable)
	FfiDestroyerBool{}.Destroy(r.Exists)
}

type FfiConverterUtxo struct{}

var FfiConverterUtxoINSTANCE = FfiConverterUtxo{}

func (c FfiConverterUtxo) Lift(rb RustBufferI) Utxo {
	return LiftFromRustBuffer[Utxo](c, rb)
}

func (c FfiConverterUtxo) Read(reader io.Reader) Utxo {
	return Utxo{
		FfiConverterOutpointINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterBoolINSTANCE.Read(reader),
		FfiConverterBoolINSTANCE.Read(reader),
	}
}

func (c FfiConverterUtxo) Lower(value Utxo) C.RustBuffer {
	return LowerIntoRustBuffer[Utxo](c, value)
}

func (c FfiConverterUtxo) Write(writer io.Writer, value Utxo) {
	FfiConverterOutpointINSTANCE.Write(writer, value.Outpoint)
	FfiConverterUint64INSTANCE.Write(writer, value.BtcAmount)
	FfiConverterBoolINSTANCE.Write(writer, value.Colorable)
	FfiConverterBoolINSTANCE.Write(writer, value.Exists)
}

type FfiDestroyerUtxo struct{}

func (_ FfiDestroyerUtxo) Destroy(value Utxo) {
	value.Destroy()
}

type WalletData struct {
	DataDir               string
	BitcoinNetwork        BitcoinNetwork
	DatabaseType          DatabaseType
	MaxAllocationsPerUtxo uint32
	AccountXpubVanilla    string
	AccountXpubColored    string
	Mnemonic              *string
	MasterFingerprint     string
	VanillaKeychain       *uint8
	SupportedSchemas      []AssetSchema
}

func (r *WalletData) Destroy() {
	FfiDestroyerString{}.Destroy(r.DataDir)
	FfiDestroyerBitcoinNetwork{}.Destroy(r.BitcoinNetwork)
	FfiDestroyerDatabaseType{}.Destroy(r.DatabaseType)
	FfiDestroyerUint32{}.Destroy(r.MaxAllocationsPerUtxo)
	FfiDestroyerString{}.Destroy(r.AccountXpubVanilla)
	FfiDestroyerString{}.Destroy(r.AccountXpubColored)
	FfiDestroyerOptionalString{}.Destroy(r.Mnemonic)
	FfiDestroyerString{}.Destroy(r.MasterFingerprint)
	FfiDestroyerOptionalUint8{}.Destroy(r.VanillaKeychain)
	FfiDestroyerSequenceAssetSchema{}.Destroy(r.SupportedSchemas)
}

type FfiConverterWalletData struct{}

var FfiConverterWalletDataINSTANCE = FfiConverterWalletData{}

func (c FfiConverterWalletData) Lift(rb RustBufferI) WalletData {
	return LiftFromRustBuffer[WalletData](c, rb)
}

func (c FfiConverterWalletData) Read(reader io.Reader) WalletData {
	return WalletData{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterBitcoinNetworkINSTANCE.Read(reader),
		FfiConverterDatabaseTypeINSTANCE.Read(reader),
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterOptionalUint8INSTANCE.Read(reader),
		FfiConverterSequenceAssetSchemaINSTANCE.Read(reader),
	}
}

func (c FfiConverterWalletData) Lower(value WalletData) C.RustBuffer {
	return LowerIntoRustBuffer[WalletData](c, value)
}

func (c FfiConverterWalletData) Write(writer io.Writer, value WalletData) {
	FfiConverterStringINSTANCE.Write(writer, value.DataDir)
	FfiConverterBitcoinNetworkINSTANCE.Write(writer, value.BitcoinNetwork)
	FfiConverterDatabaseTypeINSTANCE.Write(writer, value.DatabaseType)
	FfiConverterUint32INSTANCE.Write(writer, value.MaxAllocationsPerUtxo)
	FfiConverterStringINSTANCE.Write(writer, value.AccountXpubVanilla)
	FfiConverterStringINSTANCE.Write(writer, value.AccountXpubColored)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Mnemonic)
	FfiConverterStringINSTANCE.Write(writer, value.MasterFingerprint)
	FfiConverterOptionalUint8INSTANCE.Write(writer, value.VanillaKeychain)
	FfiConverterSequenceAssetSchemaINSTANCE.Write(writer, value.SupportedSchemas)
}

type FfiDestroyerWalletData struct{}

func (_ FfiDestroyerWalletData) Destroy(value WalletData) {
	value.Destroy()
}

type WitnessData struct {
	AmountSat uint64
	Blinding  *uint64
}

func (r *WitnessData) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.AmountSat)
	FfiDestroyerOptionalUint64{}.Destroy(r.Blinding)
}

type FfiConverterWitnessData struct{}

var FfiConverterWitnessDataINSTANCE = FfiConverterWitnessData{}

func (c FfiConverterWitnessData) Lift(rb RustBufferI) WitnessData {
	return LiftFromRustBuffer[WitnessData](c, rb)
}

func (c FfiConverterWitnessData) Read(reader io.Reader) WitnessData {
	return WitnessData{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterOptionalUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterWitnessData) Lower(value WitnessData) C.RustBuffer {
	return LowerIntoRustBuffer[WitnessData](c, value)
}

func (c FfiConverterWitnessData) Write(writer io.Writer, value WitnessData) {
	FfiConverterUint64INSTANCE.Write(writer, value.AmountSat)
	FfiConverterOptionalUint64INSTANCE.Write(writer, value.Blinding)
}

type FfiDestroyerWitnessData struct{}

func (_ FfiDestroyerWitnessData) Destroy(value WitnessData) {
	value.Destroy()
}

type AssetSchema uint

const (
	AssetSchemaNia AssetSchema = 1
	AssetSchemaUda AssetSchema = 2
	AssetSchemaCfa AssetSchema = 3
	AssetSchemaIfa AssetSchema = 4
)

type FfiConverterAssetSchema struct{}

var FfiConverterAssetSchemaINSTANCE = FfiConverterAssetSchema{}

func (c FfiConverterAssetSchema) Lift(rb RustBufferI) AssetSchema {
	return LiftFromRustBuffer[AssetSchema](c, rb)
}

func (c FfiConverterAssetSchema) Lower(value AssetSchema) C.RustBuffer {
	return LowerIntoRustBuffer[AssetSchema](c, value)
}
func (FfiConverterAssetSchema) Read(reader io.Reader) AssetSchema {
	id := readInt32(reader)
	return AssetSchema(id)
}

func (FfiConverterAssetSchema) Write(writer io.Writer, value AssetSchema) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerAssetSchema struct{}

func (_ FfiDestroyerAssetSchema) Destroy(value AssetSchema) {
}

type Assignment interface {
	Destroy()
}
type AssignmentFungible struct {
	Amount uint64
}

func (e AssignmentFungible) Destroy() {
	FfiDestroyerUint64{}.Destroy(e.Amount)
}

type AssignmentNonFungible struct {
}

func (e AssignmentNonFungible) Destroy() {
}

type AssignmentInflationRight struct {
	Amount uint64
}

func (e AssignmentInflationRight) Destroy() {
	FfiDestroyerUint64{}.Destroy(e.Amount)
}

type AssignmentReplaceRight struct {
}

func (e AssignmentReplaceRight) Destroy() {
}

type AssignmentAny struct {
}

func (e AssignmentAny) Destroy() {
}

type FfiConverterAssignment struct{}

var FfiConverterAssignmentINSTANCE = FfiConverterAssignment{}

func (c FfiConverterAssignment) Lift(rb RustBufferI) Assignment {
	return LiftFromRustBuffer[Assignment](c, rb)
}

func (c FfiConverterAssignment) Lower(value Assignment) C.RustBuffer {
	return LowerIntoRustBuffer[Assignment](c, value)
}
func (FfiConverterAssignment) Read(reader io.Reader) Assignment {
	id := readInt32(reader)
	switch id {
	case 1:
		return AssignmentFungible{
			FfiConverterUint64INSTANCE.Read(reader),
		}
	case 2:
		return AssignmentNonFungible{}
	case 3:
		return AssignmentInflationRight{
			FfiConverterUint64INSTANCE.Read(reader),
		}
	case 4:
		return AssignmentReplaceRight{}
	case 5:
		return AssignmentAny{}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterAssignment.Read()", id))
	}
}

func (FfiConverterAssignment) Write(writer io.Writer, value Assignment) {
	switch variant_value := value.(type) {
	case AssignmentFungible:
		writeInt32(writer, 1)
		FfiConverterUint64INSTANCE.Write(writer, variant_value.Amount)
	case AssignmentNonFungible:
		writeInt32(writer, 2)
	case AssignmentInflationRight:
		writeInt32(writer, 3)
		FfiConverterUint64INSTANCE.Write(writer, variant_value.Amount)
	case AssignmentReplaceRight:
		writeInt32(writer, 4)
	case AssignmentAny:
		writeInt32(writer, 5)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterAssignment.Write", value))
	}
}

type FfiDestroyerAssignment struct{}

func (_ FfiDestroyerAssignment) Destroy(value Assignment) {
	value.Destroy()
}

type BitcoinNetwork uint

const (
	BitcoinNetworkMainnet BitcoinNetwork = 1
	BitcoinNetworkTestnet BitcoinNetwork = 2
	BitcoinNetworkSignet  BitcoinNetwork = 3
	BitcoinNetworkRegtest BitcoinNetwork = 4
)

type FfiConverterBitcoinNetwork struct{}

var FfiConverterBitcoinNetworkINSTANCE = FfiConverterBitcoinNetwork{}

func (c FfiConverterBitcoinNetwork) Lift(rb RustBufferI) BitcoinNetwork {
	return LiftFromRustBuffer[BitcoinNetwork](c, rb)
}

func (c FfiConverterBitcoinNetwork) Lower(value BitcoinNetwork) C.RustBuffer {
	return LowerIntoRustBuffer[BitcoinNetwork](c, value)
}
func (FfiConverterBitcoinNetwork) Read(reader io.Reader) BitcoinNetwork {
	id := readInt32(reader)
	return BitcoinNetwork(id)
}

func (FfiConverterBitcoinNetwork) Write(writer io.Writer, value BitcoinNetwork) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerBitcoinNetwork struct{}

func (_ FfiDestroyerBitcoinNetwork) Destroy(value BitcoinNetwork) {
}

type DatabaseType uint

const (
	DatabaseTypeSqlite DatabaseType = 1
)

type FfiConverterDatabaseType struct{}

var FfiConverterDatabaseTypeINSTANCE = FfiConverterDatabaseType{}

func (c FfiConverterDatabaseType) Lift(rb RustBufferI) DatabaseType {
	return LiftFromRustBuffer[DatabaseType](c, rb)
}

func (c FfiConverterDatabaseType) Lower(value DatabaseType) C.RustBuffer {
	return LowerIntoRustBuffer[DatabaseType](c, value)
}
func (FfiConverterDatabaseType) Read(reader io.Reader) DatabaseType {
	id := readInt32(reader)
	return DatabaseType(id)
}

func (FfiConverterDatabaseType) Write(writer io.Writer, value DatabaseType) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerDatabaseType struct{}

func (_ FfiDestroyerDatabaseType) Destroy(value DatabaseType) {
}

type RecipientType uint

const (
	RecipientTypeBlind   RecipientType = 1
	RecipientTypeWitness RecipientType = 2
)

type FfiConverterRecipientType struct{}

var FfiConverterRecipientTypeINSTANCE = FfiConverterRecipientType{}

func (c FfiConverterRecipientType) Lift(rb RustBufferI) RecipientType {
	return LiftFromRustBuffer[RecipientType](c, rb)
}

func (c FfiConverterRecipientType) Lower(value RecipientType) C.RustBuffer {
	return LowerIntoRustBuffer[RecipientType](c, value)
}
func (FfiConverterRecipientType) Read(reader io.Reader) RecipientType {
	id := readInt32(reader)
	return RecipientType(id)
}

func (FfiConverterRecipientType) Write(writer io.Writer, value RecipientType) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerRecipientType struct{}

func (_ FfiDestroyerRecipientType) Destroy(value RecipientType) {
}

type RefreshTransferStatus uint

const (
	RefreshTransferStatusWaitingCounterparty  RefreshTransferStatus = 1
	RefreshTransferStatusWaitingConfirmations RefreshTransferStatus = 2
)

type FfiConverterRefreshTransferStatus struct{}

var FfiConverterRefreshTransferStatusINSTANCE = FfiConverterRefreshTransferStatus{}

func (c FfiConverterRefreshTransferStatus) Lift(rb RustBufferI) RefreshTransferStatus {
	return LiftFromRustBuffer[RefreshTransferStatus](c, rb)
}

func (c FfiConverterRefreshTransferStatus) Lower(value RefreshTransferStatus) C.RustBuffer {
	return LowerIntoRustBuffer[RefreshTransferStatus](c, value)
}
func (FfiConverterRefreshTransferStatus) Read(reader io.Reader) RefreshTransferStatus {
	id := readInt32(reader)
	return RefreshTransferStatus(id)
}

func (FfiConverterRefreshTransferStatus) Write(writer io.Writer, value RefreshTransferStatus) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerRefreshTransferStatus struct{}

func (_ FfiDestroyerRefreshTransferStatus) Destroy(value RefreshTransferStatus) {
}

type RgbLibError struct {
	err error
}

// Convience method to turn *RgbLibError into error
// Avoiding treating nil pointer as non nil error interface
func (err *RgbLibError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err RgbLibError) Error() string {
	return fmt.Sprintf("RgbLibError: %s", err.err.Error())
}

func (err RgbLibError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrRgbLibErrorAllocationsAlreadyAvailable = fmt.Errorf("RgbLibErrorAllocationsAlreadyAvailable")
var ErrRgbLibErrorAssetNotFound = fmt.Errorf("RgbLibErrorAssetNotFound")
var ErrRgbLibErrorBatchTransferNotFound = fmt.Errorf("RgbLibErrorBatchTransferNotFound")
var ErrRgbLibErrorBitcoinNetworkMismatch = fmt.Errorf("RgbLibErrorBitcoinNetworkMismatch")
var ErrRgbLibErrorCannotChangeOnline = fmt.Errorf("RgbLibErrorCannotChangeOnline")
var ErrRgbLibErrorCannotDeleteBatchTransfer = fmt.Errorf("RgbLibErrorCannotDeleteBatchTransfer")
var ErrRgbLibErrorCannotEstimateFees = fmt.Errorf("RgbLibErrorCannotEstimateFees")
var ErrRgbLibErrorCannotFailBatchTransfer = fmt.Errorf("RgbLibErrorCannotFailBatchTransfer")
var ErrRgbLibErrorCannotFinalizePsbt = fmt.Errorf("RgbLibErrorCannotFinalizePsbt")
var ErrRgbLibErrorCannotUseIfaOnMainnet = fmt.Errorf("RgbLibErrorCannotUseIfaOnMainnet")
var ErrRgbLibErrorEmptyFile = fmt.Errorf("RgbLibErrorEmptyFile")
var ErrRgbLibErrorFailedBdkSync = fmt.Errorf("RgbLibErrorFailedBdkSync")
var ErrRgbLibErrorFailedBroadcast = fmt.Errorf("RgbLibErrorFailedBroadcast")
var ErrRgbLibErrorFailedIssuance = fmt.Errorf("RgbLibErrorFailedIssuance")
var ErrRgbLibErrorFileAlreadyExists = fmt.Errorf("RgbLibErrorFileAlreadyExists")
var ErrRgbLibErrorFingerprintMismatch = fmt.Errorf("RgbLibErrorFingerprintMismatch")
var ErrRgbLibErrorIo = fmt.Errorf("RgbLibErrorIo")
var ErrRgbLibErrorInconsistency = fmt.Errorf("RgbLibErrorInconsistency")
var ErrRgbLibErrorIndexer = fmt.Errorf("RgbLibErrorIndexer")
var ErrRgbLibErrorInexistentDataDir = fmt.Errorf("RgbLibErrorInexistentDataDir")
var ErrRgbLibErrorInsufficientAllocationSlots = fmt.Errorf("RgbLibErrorInsufficientAllocationSlots")
var ErrRgbLibErrorInsufficientAssignments = fmt.Errorf("RgbLibErrorInsufficientAssignments")
var ErrRgbLibErrorInsufficientBitcoins = fmt.Errorf("RgbLibErrorInsufficientBitcoins")
var ErrRgbLibErrorInternal = fmt.Errorf("RgbLibErrorInternal")
var ErrRgbLibErrorInvalidAddress = fmt.Errorf("RgbLibErrorInvalidAddress")
var ErrRgbLibErrorInvalidAmountZero = fmt.Errorf("RgbLibErrorInvalidAmountZero")
var ErrRgbLibErrorInvalidAssetId = fmt.Errorf("RgbLibErrorInvalidAssetId")
var ErrRgbLibErrorInvalidAssignment = fmt.Errorf("RgbLibErrorInvalidAssignment")
var ErrRgbLibErrorInvalidAttachments = fmt.Errorf("RgbLibErrorInvalidAttachments")
var ErrRgbLibErrorInvalidBitcoinKeys = fmt.Errorf("RgbLibErrorInvalidBitcoinKeys")
var ErrRgbLibErrorInvalidBitcoinNetwork = fmt.Errorf("RgbLibErrorInvalidBitcoinNetwork")
var ErrRgbLibErrorInvalidColoringInfo = fmt.Errorf("RgbLibErrorInvalidColoringInfo")
var ErrRgbLibErrorInvalidConsignment = fmt.Errorf("RgbLibErrorInvalidConsignment")
var ErrRgbLibErrorInvalidDetails = fmt.Errorf("RgbLibErrorInvalidDetails")
var ErrRgbLibErrorInvalidElectrum = fmt.Errorf("RgbLibErrorInvalidElectrum")
var ErrRgbLibErrorInvalidEstimationBlocks = fmt.Errorf("RgbLibErrorInvalidEstimationBlocks")
var ErrRgbLibErrorInvalidFeeRate = fmt.Errorf("RgbLibErrorInvalidFeeRate")
var ErrRgbLibErrorInvalidFilePath = fmt.Errorf("RgbLibErrorInvalidFilePath")
var ErrRgbLibErrorInvalidFingerprint = fmt.Errorf("RgbLibErrorInvalidFingerprint")
var ErrRgbLibErrorInvalidIndexer = fmt.Errorf("RgbLibErrorInvalidIndexer")
var ErrRgbLibErrorInvalidInvoice = fmt.Errorf("RgbLibErrorInvalidInvoice")
var ErrRgbLibErrorInvalidMnemonic = fmt.Errorf("RgbLibErrorInvalidMnemonic")
var ErrRgbLibErrorInvalidName = fmt.Errorf("RgbLibErrorInvalidName")
var ErrRgbLibErrorInvalidPrecision = fmt.Errorf("RgbLibErrorInvalidPrecision")
var ErrRgbLibErrorInvalidProxyProtocol = fmt.Errorf("RgbLibErrorInvalidProxyProtocol")
var ErrRgbLibErrorInvalidPsbt = fmt.Errorf("RgbLibErrorInvalidPsbt")
var ErrRgbLibErrorInvalidPubkey = fmt.Errorf("RgbLibErrorInvalidPubkey")
var ErrRgbLibErrorInvalidRecipientData = fmt.Errorf("RgbLibErrorInvalidRecipientData")
var ErrRgbLibErrorInvalidRecipientId = fmt.Errorf("RgbLibErrorInvalidRecipientId")
var ErrRgbLibErrorInvalidRecipientNetwork = fmt.Errorf("RgbLibErrorInvalidRecipientNetwork")
var ErrRgbLibErrorInvalidTicker = fmt.Errorf("RgbLibErrorInvalidTicker")
var ErrRgbLibErrorInvalidTransportEndpoint = fmt.Errorf("RgbLibErrorInvalidTransportEndpoint")
var ErrRgbLibErrorInvalidTransportEndpoints = fmt.Errorf("RgbLibErrorInvalidTransportEndpoints")
var ErrRgbLibErrorInvalidTxid = fmt.Errorf("RgbLibErrorInvalidTxid")
var ErrRgbLibErrorInvalidVanillaKeychain = fmt.Errorf("RgbLibErrorInvalidVanillaKeychain")
var ErrRgbLibErrorMaxFeeExceeded = fmt.Errorf("RgbLibErrorMaxFeeExceeded")
var ErrRgbLibErrorMinFeeNotMet = fmt.Errorf("RgbLibErrorMinFeeNotMet")
var ErrRgbLibErrorNetwork = fmt.Errorf("RgbLibErrorNetwork")
var ErrRgbLibErrorNoConsignment = fmt.Errorf("RgbLibErrorNoConsignment")
var ErrRgbLibErrorNoIssuanceAmounts = fmt.Errorf("RgbLibErrorNoIssuanceAmounts")
var ErrRgbLibErrorNoSupportedSchemas = fmt.Errorf("RgbLibErrorNoSupportedSchemas")
var ErrRgbLibErrorNoValidTransportEndpoint = fmt.Errorf("RgbLibErrorNoValidTransportEndpoint")
var ErrRgbLibErrorOffline = fmt.Errorf("RgbLibErrorOffline")
var ErrRgbLibErrorOnlineNeeded = fmt.Errorf("RgbLibErrorOnlineNeeded")
var ErrRgbLibErrorOutputBelowDustLimit = fmt.Errorf("RgbLibErrorOutputBelowDustLimit")
var ErrRgbLibErrorProxy = fmt.Errorf("RgbLibErrorProxy")
var ErrRgbLibErrorRecipientIdAlreadyUsed = fmt.Errorf("RgbLibErrorRecipientIdAlreadyUsed")
var ErrRgbLibErrorRecipientIdDuplicated = fmt.Errorf("RgbLibErrorRecipientIdDuplicated")
var ErrRgbLibErrorTooHighInflationAmounts = fmt.Errorf("RgbLibErrorTooHighInflationAmounts")
var ErrRgbLibErrorTooHighIssuanceAmounts = fmt.Errorf("RgbLibErrorTooHighIssuanceAmounts")
var ErrRgbLibErrorUnknownRgbSchema = fmt.Errorf("RgbLibErrorUnknownRgbSchema")
var ErrRgbLibErrorUnsupportedBackupVersion = fmt.Errorf("RgbLibErrorUnsupportedBackupVersion")
var ErrRgbLibErrorUnsupportedLayer1 = fmt.Errorf("RgbLibErrorUnsupportedLayer1")
var ErrRgbLibErrorUnsupportedSchema = fmt.Errorf("RgbLibErrorUnsupportedSchema")
var ErrRgbLibErrorUnsupportedTransportType = fmt.Errorf("RgbLibErrorUnsupportedTransportType")
var ErrRgbLibErrorWalletDirAlreadyExists = fmt.Errorf("RgbLibErrorWalletDirAlreadyExists")
var ErrRgbLibErrorWatchOnly = fmt.Errorf("RgbLibErrorWatchOnly")
var ErrRgbLibErrorWrongPassword = fmt.Errorf("RgbLibErrorWrongPassword")

// Variant structs
type RgbLibErrorAllocationsAlreadyAvailable struct {
}

func NewRgbLibErrorAllocationsAlreadyAvailable() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorAllocationsAlreadyAvailable{}}
}

func (e RgbLibErrorAllocationsAlreadyAvailable) destroy() {
}

func (err RgbLibErrorAllocationsAlreadyAvailable) Error() string {
	return fmt.Sprint("AllocationsAlreadyAvailable")
}

func (self RgbLibErrorAllocationsAlreadyAvailable) Is(target error) bool {
	return target == ErrRgbLibErrorAllocationsAlreadyAvailable
}

type RgbLibErrorAssetNotFound struct {
	AssetId string
}

func NewRgbLibErrorAssetNotFound(
	assetId string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorAssetNotFound{
		AssetId: assetId}}
}

func (e RgbLibErrorAssetNotFound) destroy() {
	FfiDestroyerString{}.Destroy(e.AssetId)
}

func (err RgbLibErrorAssetNotFound) Error() string {
	return fmt.Sprint("AssetNotFound",
		": ",

		"AssetId=",
		err.AssetId,
	)
}

func (self RgbLibErrorAssetNotFound) Is(target error) bool {
	return target == ErrRgbLibErrorAssetNotFound
}

type RgbLibErrorBatchTransferNotFound struct {
	Idx int32
}

func NewRgbLibErrorBatchTransferNotFound(
	idx int32,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorBatchTransferNotFound{
		Idx: idx}}
}

func (e RgbLibErrorBatchTransferNotFound) destroy() {
	FfiDestroyerInt32{}.Destroy(e.Idx)
}

func (err RgbLibErrorBatchTransferNotFound) Error() string {
	return fmt.Sprint("BatchTransferNotFound",
		": ",

		"Idx=",
		err.Idx,
	)
}

func (self RgbLibErrorBatchTransferNotFound) Is(target error) bool {
	return target == ErrRgbLibErrorBatchTransferNotFound
}

type RgbLibErrorBitcoinNetworkMismatch struct {
}

func NewRgbLibErrorBitcoinNetworkMismatch() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorBitcoinNetworkMismatch{}}
}

func (e RgbLibErrorBitcoinNetworkMismatch) destroy() {
}

func (err RgbLibErrorBitcoinNetworkMismatch) Error() string {
	return fmt.Sprint("BitcoinNetworkMismatch")
}

func (self RgbLibErrorBitcoinNetworkMismatch) Is(target error) bool {
	return target == ErrRgbLibErrorBitcoinNetworkMismatch
}

type RgbLibErrorCannotChangeOnline struct {
}

func NewRgbLibErrorCannotChangeOnline() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorCannotChangeOnline{}}
}

func (e RgbLibErrorCannotChangeOnline) destroy() {
}

func (err RgbLibErrorCannotChangeOnline) Error() string {
	return fmt.Sprint("CannotChangeOnline")
}

func (self RgbLibErrorCannotChangeOnline) Is(target error) bool {
	return target == ErrRgbLibErrorCannotChangeOnline
}

type RgbLibErrorCannotDeleteBatchTransfer struct {
}

func NewRgbLibErrorCannotDeleteBatchTransfer() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorCannotDeleteBatchTransfer{}}
}

func (e RgbLibErrorCannotDeleteBatchTransfer) destroy() {
}

func (err RgbLibErrorCannotDeleteBatchTransfer) Error() string {
	return fmt.Sprint("CannotDeleteBatchTransfer")
}

func (self RgbLibErrorCannotDeleteBatchTransfer) Is(target error) bool {
	return target == ErrRgbLibErrorCannotDeleteBatchTransfer
}

type RgbLibErrorCannotEstimateFees struct {
}

func NewRgbLibErrorCannotEstimateFees() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorCannotEstimateFees{}}
}

func (e RgbLibErrorCannotEstimateFees) destroy() {
}

func (err RgbLibErrorCannotEstimateFees) Error() string {
	return fmt.Sprint("CannotEstimateFees")
}

func (self RgbLibErrorCannotEstimateFees) Is(target error) bool {
	return target == ErrRgbLibErrorCannotEstimateFees
}

type RgbLibErrorCannotFailBatchTransfer struct {
}

func NewRgbLibErrorCannotFailBatchTransfer() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorCannotFailBatchTransfer{}}
}

func (e RgbLibErrorCannotFailBatchTransfer) destroy() {
}

func (err RgbLibErrorCannotFailBatchTransfer) Error() string {
	return fmt.Sprint("CannotFailBatchTransfer")
}

func (self RgbLibErrorCannotFailBatchTransfer) Is(target error) bool {
	return target == ErrRgbLibErrorCannotFailBatchTransfer
}

type RgbLibErrorCannotFinalizePsbt struct {
}

func NewRgbLibErrorCannotFinalizePsbt() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorCannotFinalizePsbt{}}
}

func (e RgbLibErrorCannotFinalizePsbt) destroy() {
}

func (err RgbLibErrorCannotFinalizePsbt) Error() string {
	return fmt.Sprint("CannotFinalizePsbt")
}

func (self RgbLibErrorCannotFinalizePsbt) Is(target error) bool {
	return target == ErrRgbLibErrorCannotFinalizePsbt
}

type RgbLibErrorCannotUseIfaOnMainnet struct {
}

func NewRgbLibErrorCannotUseIfaOnMainnet() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorCannotUseIfaOnMainnet{}}
}

func (e RgbLibErrorCannotUseIfaOnMainnet) destroy() {
}

func (err RgbLibErrorCannotUseIfaOnMainnet) Error() string {
	return fmt.Sprint("CannotUseIfaOnMainnet")
}

func (self RgbLibErrorCannotUseIfaOnMainnet) Is(target error) bool {
	return target == ErrRgbLibErrorCannotUseIfaOnMainnet
}

type RgbLibErrorEmptyFile struct {
	FilePath string
}

func NewRgbLibErrorEmptyFile(
	filePath string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorEmptyFile{
		FilePath: filePath}}
}

func (e RgbLibErrorEmptyFile) destroy() {
	FfiDestroyerString{}.Destroy(e.FilePath)
}

func (err RgbLibErrorEmptyFile) Error() string {
	return fmt.Sprint("EmptyFile",
		": ",

		"FilePath=",
		err.FilePath,
	)
}

func (self RgbLibErrorEmptyFile) Is(target error) bool {
	return target == ErrRgbLibErrorEmptyFile
}

type RgbLibErrorFailedBdkSync struct {
	Details string
}

func NewRgbLibErrorFailedBdkSync(
	details string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorFailedBdkSync{
		Details: details}}
}

func (e RgbLibErrorFailedBdkSync) destroy() {
	FfiDestroyerString{}.Destroy(e.Details)
}

func (err RgbLibErrorFailedBdkSync) Error() string {
	return fmt.Sprint("FailedBdkSync",
		": ",

		"Details=",
		err.Details,
	)
}

func (self RgbLibErrorFailedBdkSync) Is(target error) bool {
	return target == ErrRgbLibErrorFailedBdkSync
}

type RgbLibErrorFailedBroadcast struct {
	Details string
}

func NewRgbLibErrorFailedBroadcast(
	details string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorFailedBroadcast{
		Details: details}}
}

func (e RgbLibErrorFailedBroadcast) destroy() {
	FfiDestroyerString{}.Destroy(e.Details)
}

func (err RgbLibErrorFailedBroadcast) Error() string {
	return fmt.Sprint("FailedBroadcast",
		": ",

		"Details=",
		err.Details,
	)
}

func (self RgbLibErrorFailedBroadcast) Is(target error) bool {
	return target == ErrRgbLibErrorFailedBroadcast
}

type RgbLibErrorFailedIssuance struct {
	Details string
}

func NewRgbLibErrorFailedIssuance(
	details string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorFailedIssuance{
		Details: details}}
}

func (e RgbLibErrorFailedIssuance) destroy() {
	FfiDestroyerString{}.Destroy(e.Details)
}

func (err RgbLibErrorFailedIssuance) Error() string {
	return fmt.Sprint("FailedIssuance",
		": ",

		"Details=",
		err.Details,
	)
}

func (self RgbLibErrorFailedIssuance) Is(target error) bool {
	return target == ErrRgbLibErrorFailedIssuance
}

type RgbLibErrorFileAlreadyExists struct {
	Path string
}

func NewRgbLibErrorFileAlreadyExists(
	path string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorFileAlreadyExists{
		Path: path}}
}

func (e RgbLibErrorFileAlreadyExists) destroy() {
	FfiDestroyerString{}.Destroy(e.Path)
}

func (err RgbLibErrorFileAlreadyExists) Error() string {
	return fmt.Sprint("FileAlreadyExists",
		": ",

		"Path=",
		err.Path,
	)
}

func (self RgbLibErrorFileAlreadyExists) Is(target error) bool {
	return target == ErrRgbLibErrorFileAlreadyExists
}

type RgbLibErrorFingerprintMismatch struct {
}

func NewRgbLibErrorFingerprintMismatch() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorFingerprintMismatch{}}
}

func (e RgbLibErrorFingerprintMismatch) destroy() {
}

func (err RgbLibErrorFingerprintMismatch) Error() string {
	return fmt.Sprint("FingerprintMismatch")
}

func (self RgbLibErrorFingerprintMismatch) Is(target error) bool {
	return target == ErrRgbLibErrorFingerprintMismatch
}

type RgbLibErrorIo struct {
	Details string
}

func NewRgbLibErrorIo(
	details string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorIo{
		Details: details}}
}

func (e RgbLibErrorIo) destroy() {
	FfiDestroyerString{}.Destroy(e.Details)
}

func (err RgbLibErrorIo) Error() string {
	return fmt.Sprint("Io",
		": ",

		"Details=",
		err.Details,
	)
}

func (self RgbLibErrorIo) Is(target error) bool {
	return target == ErrRgbLibErrorIo
}

type RgbLibErrorInconsistency struct {
	Details string
}

func NewRgbLibErrorInconsistency(
	details string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInconsistency{
		Details: details}}
}

func (e RgbLibErrorInconsistency) destroy() {
	FfiDestroyerString{}.Destroy(e.Details)
}

func (err RgbLibErrorInconsistency) Error() string {
	return fmt.Sprint("Inconsistency",
		": ",

		"Details=",
		err.Details,
	)
}

func (self RgbLibErrorInconsistency) Is(target error) bool {
	return target == ErrRgbLibErrorInconsistency
}

type RgbLibErrorIndexer struct {
	Details string
}

func NewRgbLibErrorIndexer(
	details string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorIndexer{
		Details: details}}
}

func (e RgbLibErrorIndexer) destroy() {
	FfiDestroyerString{}.Destroy(e.Details)
}

func (err RgbLibErrorIndexer) Error() string {
	return fmt.Sprint("Indexer",
		": ",

		"Details=",
		err.Details,
	)
}

func (self RgbLibErrorIndexer) Is(target error) bool {
	return target == ErrRgbLibErrorIndexer
}

type RgbLibErrorInexistentDataDir struct {
}

func NewRgbLibErrorInexistentDataDir() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInexistentDataDir{}}
}

func (e RgbLibErrorInexistentDataDir) destroy() {
}

func (err RgbLibErrorInexistentDataDir) Error() string {
	return fmt.Sprint("InexistentDataDir")
}

func (self RgbLibErrorInexistentDataDir) Is(target error) bool {
	return target == ErrRgbLibErrorInexistentDataDir
}

type RgbLibErrorInsufficientAllocationSlots struct {
}

func NewRgbLibErrorInsufficientAllocationSlots() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInsufficientAllocationSlots{}}
}

func (e RgbLibErrorInsufficientAllocationSlots) destroy() {
}

func (err RgbLibErrorInsufficientAllocationSlots) Error() string {
	return fmt.Sprint("InsufficientAllocationSlots")
}

func (self RgbLibErrorInsufficientAllocationSlots) Is(target error) bool {
	return target == ErrRgbLibErrorInsufficientAllocationSlots
}

type RgbLibErrorInsufficientAssignments struct {
	AssetId   string
	Available AssignmentsCollection
}

func NewRgbLibErrorInsufficientAssignments(
	assetId string,
	available AssignmentsCollection,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInsufficientAssignments{
		AssetId:   assetId,
		Available: available}}
}

func (e RgbLibErrorInsufficientAssignments) destroy() {
	FfiDestroyerString{}.Destroy(e.AssetId)
	FfiDestroyerAssignmentsCollection{}.Destroy(e.Available)
}

func (err RgbLibErrorInsufficientAssignments) Error() string {
	return fmt.Sprint("InsufficientAssignments",
		": ",

		"AssetId=",
		err.AssetId,
		", ",
		"Available=",
		err.Available,
	)
}

func (self RgbLibErrorInsufficientAssignments) Is(target error) bool {
	return target == ErrRgbLibErrorInsufficientAssignments
}

type RgbLibErrorInsufficientBitcoins struct {
	Needed    uint64
	Available uint64
}

func NewRgbLibErrorInsufficientBitcoins(
	needed uint64,
	available uint64,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInsufficientBitcoins{
		Needed:    needed,
		Available: available}}
}

func (e RgbLibErrorInsufficientBitcoins) destroy() {
	FfiDestroyerUint64{}.Destroy(e.Needed)
	FfiDestroyerUint64{}.Destroy(e.Available)
}

func (err RgbLibErrorInsufficientBitcoins) Error() string {
	return fmt.Sprint("InsufficientBitcoins",
		": ",

		"Needed=",
		err.Needed,
		", ",
		"Available=",
		err.Available,
	)
}

func (self RgbLibErrorInsufficientBitcoins) Is(target error) bool {
	return target == ErrRgbLibErrorInsufficientBitcoins
}

type RgbLibErrorInternal struct {
	Details string
}

func NewRgbLibErrorInternal(
	details string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInternal{
		Details: details}}
}

func (e RgbLibErrorInternal) destroy() {
	FfiDestroyerString{}.Destroy(e.Details)
}

func (err RgbLibErrorInternal) Error() string {
	return fmt.Sprint("Internal",
		": ",

		"Details=",
		err.Details,
	)
}

func (self RgbLibErrorInternal) Is(target error) bool {
	return target == ErrRgbLibErrorInternal
}

type RgbLibErrorInvalidAddress struct {
	Details string
}

func NewRgbLibErrorInvalidAddress(
	details string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidAddress{
		Details: details}}
}

func (e RgbLibErrorInvalidAddress) destroy() {
	FfiDestroyerString{}.Destroy(e.Details)
}

func (err RgbLibErrorInvalidAddress) Error() string {
	return fmt.Sprint("InvalidAddress",
		": ",

		"Details=",
		err.Details,
	)
}

func (self RgbLibErrorInvalidAddress) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidAddress
}

type RgbLibErrorInvalidAmountZero struct {
}

func NewRgbLibErrorInvalidAmountZero() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidAmountZero{}}
}

func (e RgbLibErrorInvalidAmountZero) destroy() {
}

func (err RgbLibErrorInvalidAmountZero) Error() string {
	return fmt.Sprint("InvalidAmountZero")
}

func (self RgbLibErrorInvalidAmountZero) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidAmountZero
}

type RgbLibErrorInvalidAssetId struct {
	AssetId string
}

func NewRgbLibErrorInvalidAssetId(
	assetId string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidAssetId{
		AssetId: assetId}}
}

func (e RgbLibErrorInvalidAssetId) destroy() {
	FfiDestroyerString{}.Destroy(e.AssetId)
}

func (err RgbLibErrorInvalidAssetId) Error() string {
	return fmt.Sprint("InvalidAssetId",
		": ",

		"AssetId=",
		err.AssetId,
	)
}

func (self RgbLibErrorInvalidAssetId) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidAssetId
}

type RgbLibErrorInvalidAssignment struct {
}

func NewRgbLibErrorInvalidAssignment() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidAssignment{}}
}

func (e RgbLibErrorInvalidAssignment) destroy() {
}

func (err RgbLibErrorInvalidAssignment) Error() string {
	return fmt.Sprint("InvalidAssignment")
}

func (self RgbLibErrorInvalidAssignment) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidAssignment
}

type RgbLibErrorInvalidAttachments struct {
	Details string
}

func NewRgbLibErrorInvalidAttachments(
	details string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidAttachments{
		Details: details}}
}

func (e RgbLibErrorInvalidAttachments) destroy() {
	FfiDestroyerString{}.Destroy(e.Details)
}

func (err RgbLibErrorInvalidAttachments) Error() string {
	return fmt.Sprint("InvalidAttachments",
		": ",

		"Details=",
		err.Details,
	)
}

func (self RgbLibErrorInvalidAttachments) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidAttachments
}

type RgbLibErrorInvalidBitcoinKeys struct {
}

func NewRgbLibErrorInvalidBitcoinKeys() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidBitcoinKeys{}}
}

func (e RgbLibErrorInvalidBitcoinKeys) destroy() {
}

func (err RgbLibErrorInvalidBitcoinKeys) Error() string {
	return fmt.Sprint("InvalidBitcoinKeys")
}

func (self RgbLibErrorInvalidBitcoinKeys) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidBitcoinKeys
}

type RgbLibErrorInvalidBitcoinNetwork struct {
	Network string
}

func NewRgbLibErrorInvalidBitcoinNetwork(
	network string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidBitcoinNetwork{
		Network: network}}
}

func (e RgbLibErrorInvalidBitcoinNetwork) destroy() {
	FfiDestroyerString{}.Destroy(e.Network)
}

func (err RgbLibErrorInvalidBitcoinNetwork) Error() string {
	return fmt.Sprint("InvalidBitcoinNetwork",
		": ",

		"Network=",
		err.Network,
	)
}

func (self RgbLibErrorInvalidBitcoinNetwork) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidBitcoinNetwork
}

type RgbLibErrorInvalidColoringInfo struct {
	Details string
}

func NewRgbLibErrorInvalidColoringInfo(
	details string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidColoringInfo{
		Details: details}}
}

func (e RgbLibErrorInvalidColoringInfo) destroy() {
	FfiDestroyerString{}.Destroy(e.Details)
}

func (err RgbLibErrorInvalidColoringInfo) Error() string {
	return fmt.Sprint("InvalidColoringInfo",
		": ",

		"Details=",
		err.Details,
	)
}

func (self RgbLibErrorInvalidColoringInfo) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidColoringInfo
}

type RgbLibErrorInvalidConsignment struct {
}

func NewRgbLibErrorInvalidConsignment() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidConsignment{}}
}

func (e RgbLibErrorInvalidConsignment) destroy() {
}

func (err RgbLibErrorInvalidConsignment) Error() string {
	return fmt.Sprint("InvalidConsignment")
}

func (self RgbLibErrorInvalidConsignment) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidConsignment
}

type RgbLibErrorInvalidDetails struct {
	Details string
}

func NewRgbLibErrorInvalidDetails(
	details string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidDetails{
		Details: details}}
}

func (e RgbLibErrorInvalidDetails) destroy() {
	FfiDestroyerString{}.Destroy(e.Details)
}

func (err RgbLibErrorInvalidDetails) Error() string {
	return fmt.Sprint("InvalidDetails",
		": ",

		"Details=",
		err.Details,
	)
}

func (self RgbLibErrorInvalidDetails) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidDetails
}

type RgbLibErrorInvalidElectrum struct {
	Details string
}

func NewRgbLibErrorInvalidElectrum(
	details string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidElectrum{
		Details: details}}
}

func (e RgbLibErrorInvalidElectrum) destroy() {
	FfiDestroyerString{}.Destroy(e.Details)
}

func (err RgbLibErrorInvalidElectrum) Error() string {
	return fmt.Sprint("InvalidElectrum",
		": ",

		"Details=",
		err.Details,
	)
}

func (self RgbLibErrorInvalidElectrum) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidElectrum
}

type RgbLibErrorInvalidEstimationBlocks struct {
}

func NewRgbLibErrorInvalidEstimationBlocks() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidEstimationBlocks{}}
}

func (e RgbLibErrorInvalidEstimationBlocks) destroy() {
}

func (err RgbLibErrorInvalidEstimationBlocks) Error() string {
	return fmt.Sprint("InvalidEstimationBlocks")
}

func (self RgbLibErrorInvalidEstimationBlocks) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidEstimationBlocks
}

type RgbLibErrorInvalidFeeRate struct {
	Details string
}

func NewRgbLibErrorInvalidFeeRate(
	details string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidFeeRate{
		Details: details}}
}

func (e RgbLibErrorInvalidFeeRate) destroy() {
	FfiDestroyerString{}.Destroy(e.Details)
}

func (err RgbLibErrorInvalidFeeRate) Error() string {
	return fmt.Sprint("InvalidFeeRate",
		": ",

		"Details=",
		err.Details,
	)
}

func (self RgbLibErrorInvalidFeeRate) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidFeeRate
}

type RgbLibErrorInvalidFilePath struct {
	FilePath string
}

func NewRgbLibErrorInvalidFilePath(
	filePath string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidFilePath{
		FilePath: filePath}}
}

func (e RgbLibErrorInvalidFilePath) destroy() {
	FfiDestroyerString{}.Destroy(e.FilePath)
}

func (err RgbLibErrorInvalidFilePath) Error() string {
	return fmt.Sprint("InvalidFilePath",
		": ",

		"FilePath=",
		err.FilePath,
	)
}

func (self RgbLibErrorInvalidFilePath) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidFilePath
}

type RgbLibErrorInvalidFingerprint struct {
}

func NewRgbLibErrorInvalidFingerprint() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidFingerprint{}}
}

func (e RgbLibErrorInvalidFingerprint) destroy() {
}

func (err RgbLibErrorInvalidFingerprint) Error() string {
	return fmt.Sprint("InvalidFingerprint")
}

func (self RgbLibErrorInvalidFingerprint) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidFingerprint
}

type RgbLibErrorInvalidIndexer struct {
	Details string
}

func NewRgbLibErrorInvalidIndexer(
	details string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidIndexer{
		Details: details}}
}

func (e RgbLibErrorInvalidIndexer) destroy() {
	FfiDestroyerString{}.Destroy(e.Details)
}

func (err RgbLibErrorInvalidIndexer) Error() string {
	return fmt.Sprint("InvalidIndexer",
		": ",

		"Details=",
		err.Details,
	)
}

func (self RgbLibErrorInvalidIndexer) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidIndexer
}

type RgbLibErrorInvalidInvoice struct {
	Details string
}

func NewRgbLibErrorInvalidInvoice(
	details string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidInvoice{
		Details: details}}
}

func (e RgbLibErrorInvalidInvoice) destroy() {
	FfiDestroyerString{}.Destroy(e.Details)
}

func (err RgbLibErrorInvalidInvoice) Error() string {
	return fmt.Sprint("InvalidInvoice",
		": ",

		"Details=",
		err.Details,
	)
}

func (self RgbLibErrorInvalidInvoice) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidInvoice
}

type RgbLibErrorInvalidMnemonic struct {
	Details string
}

func NewRgbLibErrorInvalidMnemonic(
	details string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidMnemonic{
		Details: details}}
}

func (e RgbLibErrorInvalidMnemonic) destroy() {
	FfiDestroyerString{}.Destroy(e.Details)
}

func (err RgbLibErrorInvalidMnemonic) Error() string {
	return fmt.Sprint("InvalidMnemonic",
		": ",

		"Details=",
		err.Details,
	)
}

func (self RgbLibErrorInvalidMnemonic) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidMnemonic
}

type RgbLibErrorInvalidName struct {
	Details string
}

func NewRgbLibErrorInvalidName(
	details string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidName{
		Details: details}}
}

func (e RgbLibErrorInvalidName) destroy() {
	FfiDestroyerString{}.Destroy(e.Details)
}

func (err RgbLibErrorInvalidName) Error() string {
	return fmt.Sprint("InvalidName",
		": ",

		"Details=",
		err.Details,
	)
}

func (self RgbLibErrorInvalidName) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidName
}

type RgbLibErrorInvalidPrecision struct {
	Details string
}

func NewRgbLibErrorInvalidPrecision(
	details string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidPrecision{
		Details: details}}
}

func (e RgbLibErrorInvalidPrecision) destroy() {
	FfiDestroyerString{}.Destroy(e.Details)
}

func (err RgbLibErrorInvalidPrecision) Error() string {
	return fmt.Sprint("InvalidPrecision",
		": ",

		"Details=",
		err.Details,
	)
}

func (self RgbLibErrorInvalidPrecision) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidPrecision
}

type RgbLibErrorInvalidProxyProtocol struct {
	Version string
}

func NewRgbLibErrorInvalidProxyProtocol(
	version string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidProxyProtocol{
		Version: version}}
}

func (e RgbLibErrorInvalidProxyProtocol) destroy() {
	FfiDestroyerString{}.Destroy(e.Version)
}

func (err RgbLibErrorInvalidProxyProtocol) Error() string {
	return fmt.Sprint("InvalidProxyProtocol",
		": ",

		"Version=",
		err.Version,
	)
}

func (self RgbLibErrorInvalidProxyProtocol) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidProxyProtocol
}

type RgbLibErrorInvalidPsbt struct {
	Details string
}

func NewRgbLibErrorInvalidPsbt(
	details string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidPsbt{
		Details: details}}
}

func (e RgbLibErrorInvalidPsbt) destroy() {
	FfiDestroyerString{}.Destroy(e.Details)
}

func (err RgbLibErrorInvalidPsbt) Error() string {
	return fmt.Sprint("InvalidPsbt",
		": ",

		"Details=",
		err.Details,
	)
}

func (self RgbLibErrorInvalidPsbt) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidPsbt
}

type RgbLibErrorInvalidPubkey struct {
	Details string
}

func NewRgbLibErrorInvalidPubkey(
	details string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidPubkey{
		Details: details}}
}

func (e RgbLibErrorInvalidPubkey) destroy() {
	FfiDestroyerString{}.Destroy(e.Details)
}

func (err RgbLibErrorInvalidPubkey) Error() string {
	return fmt.Sprint("InvalidPubkey",
		": ",

		"Details=",
		err.Details,
	)
}

func (self RgbLibErrorInvalidPubkey) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidPubkey
}

type RgbLibErrorInvalidRecipientData struct {
	Details string
}

func NewRgbLibErrorInvalidRecipientData(
	details string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidRecipientData{
		Details: details}}
}

func (e RgbLibErrorInvalidRecipientData) destroy() {
	FfiDestroyerString{}.Destroy(e.Details)
}

func (err RgbLibErrorInvalidRecipientData) Error() string {
	return fmt.Sprint("InvalidRecipientData",
		": ",

		"Details=",
		err.Details,
	)
}

func (self RgbLibErrorInvalidRecipientData) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidRecipientData
}

type RgbLibErrorInvalidRecipientId struct {
}

func NewRgbLibErrorInvalidRecipientId() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidRecipientId{}}
}

func (e RgbLibErrorInvalidRecipientId) destroy() {
}

func (err RgbLibErrorInvalidRecipientId) Error() string {
	return fmt.Sprint("InvalidRecipientId")
}

func (self RgbLibErrorInvalidRecipientId) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidRecipientId
}

type RgbLibErrorInvalidRecipientNetwork struct {
}

func NewRgbLibErrorInvalidRecipientNetwork() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidRecipientNetwork{}}
}

func (e RgbLibErrorInvalidRecipientNetwork) destroy() {
}

func (err RgbLibErrorInvalidRecipientNetwork) Error() string {
	return fmt.Sprint("InvalidRecipientNetwork")
}

func (self RgbLibErrorInvalidRecipientNetwork) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidRecipientNetwork
}

type RgbLibErrorInvalidTicker struct {
	Details string
}

func NewRgbLibErrorInvalidTicker(
	details string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidTicker{
		Details: details}}
}

func (e RgbLibErrorInvalidTicker) destroy() {
	FfiDestroyerString{}.Destroy(e.Details)
}

func (err RgbLibErrorInvalidTicker) Error() string {
	return fmt.Sprint("InvalidTicker",
		": ",

		"Details=",
		err.Details,
	)
}

func (self RgbLibErrorInvalidTicker) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidTicker
}

type RgbLibErrorInvalidTransportEndpoint struct {
	Details string
}

func NewRgbLibErrorInvalidTransportEndpoint(
	details string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidTransportEndpoint{
		Details: details}}
}

func (e RgbLibErrorInvalidTransportEndpoint) destroy() {
	FfiDestroyerString{}.Destroy(e.Details)
}

func (err RgbLibErrorInvalidTransportEndpoint) Error() string {
	return fmt.Sprint("InvalidTransportEndpoint",
		": ",

		"Details=",
		err.Details,
	)
}

func (self RgbLibErrorInvalidTransportEndpoint) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidTransportEndpoint
}

type RgbLibErrorInvalidTransportEndpoints struct {
	Details string
}

func NewRgbLibErrorInvalidTransportEndpoints(
	details string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidTransportEndpoints{
		Details: details}}
}

func (e RgbLibErrorInvalidTransportEndpoints) destroy() {
	FfiDestroyerString{}.Destroy(e.Details)
}

func (err RgbLibErrorInvalidTransportEndpoints) Error() string {
	return fmt.Sprint("InvalidTransportEndpoints",
		": ",

		"Details=",
		err.Details,
	)
}

func (self RgbLibErrorInvalidTransportEndpoints) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidTransportEndpoints
}

type RgbLibErrorInvalidTxid struct {
}

func NewRgbLibErrorInvalidTxid() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidTxid{}}
}

func (e RgbLibErrorInvalidTxid) destroy() {
}

func (err RgbLibErrorInvalidTxid) Error() string {
	return fmt.Sprint("InvalidTxid")
}

func (self RgbLibErrorInvalidTxid) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidTxid
}

type RgbLibErrorInvalidVanillaKeychain struct {
}

func NewRgbLibErrorInvalidVanillaKeychain() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorInvalidVanillaKeychain{}}
}

func (e RgbLibErrorInvalidVanillaKeychain) destroy() {
}

func (err RgbLibErrorInvalidVanillaKeychain) Error() string {
	return fmt.Sprint("InvalidVanillaKeychain")
}

func (self RgbLibErrorInvalidVanillaKeychain) Is(target error) bool {
	return target == ErrRgbLibErrorInvalidVanillaKeychain
}

type RgbLibErrorMaxFeeExceeded struct {
	Txid string
}

func NewRgbLibErrorMaxFeeExceeded(
	txid string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorMaxFeeExceeded{
		Txid: txid}}
}

func (e RgbLibErrorMaxFeeExceeded) destroy() {
	FfiDestroyerString{}.Destroy(e.Txid)
}

func (err RgbLibErrorMaxFeeExceeded) Error() string {
	return fmt.Sprint("MaxFeeExceeded",
		": ",

		"Txid=",
		err.Txid,
	)
}

func (self RgbLibErrorMaxFeeExceeded) Is(target error) bool {
	return target == ErrRgbLibErrorMaxFeeExceeded
}

type RgbLibErrorMinFeeNotMet struct {
	Txid string
}

func NewRgbLibErrorMinFeeNotMet(
	txid string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorMinFeeNotMet{
		Txid: txid}}
}

func (e RgbLibErrorMinFeeNotMet) destroy() {
	FfiDestroyerString{}.Destroy(e.Txid)
}

func (err RgbLibErrorMinFeeNotMet) Error() string {
	return fmt.Sprint("MinFeeNotMet",
		": ",

		"Txid=",
		err.Txid,
	)
}

func (self RgbLibErrorMinFeeNotMet) Is(target error) bool {
	return target == ErrRgbLibErrorMinFeeNotMet
}

type RgbLibErrorNetwork struct {
	Details string
}

func NewRgbLibErrorNetwork(
	details string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorNetwork{
		Details: details}}
}

func (e RgbLibErrorNetwork) destroy() {
	FfiDestroyerString{}.Destroy(e.Details)
}

func (err RgbLibErrorNetwork) Error() string {
	return fmt.Sprint("Network",
		": ",

		"Details=",
		err.Details,
	)
}

func (self RgbLibErrorNetwork) Is(target error) bool {
	return target == ErrRgbLibErrorNetwork
}

type RgbLibErrorNoConsignment struct {
}

func NewRgbLibErrorNoConsignment() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorNoConsignment{}}
}

func (e RgbLibErrorNoConsignment) destroy() {
}

func (err RgbLibErrorNoConsignment) Error() string {
	return fmt.Sprint("NoConsignment")
}

func (self RgbLibErrorNoConsignment) Is(target error) bool {
	return target == ErrRgbLibErrorNoConsignment
}

type RgbLibErrorNoIssuanceAmounts struct {
}

func NewRgbLibErrorNoIssuanceAmounts() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorNoIssuanceAmounts{}}
}

func (e RgbLibErrorNoIssuanceAmounts) destroy() {
}

func (err RgbLibErrorNoIssuanceAmounts) Error() string {
	return fmt.Sprint("NoIssuanceAmounts")
}

func (self RgbLibErrorNoIssuanceAmounts) Is(target error) bool {
	return target == ErrRgbLibErrorNoIssuanceAmounts
}

type RgbLibErrorNoSupportedSchemas struct {
}

func NewRgbLibErrorNoSupportedSchemas() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorNoSupportedSchemas{}}
}

func (e RgbLibErrorNoSupportedSchemas) destroy() {
}

func (err RgbLibErrorNoSupportedSchemas) Error() string {
	return fmt.Sprint("NoSupportedSchemas")
}

func (self RgbLibErrorNoSupportedSchemas) Is(target error) bool {
	return target == ErrRgbLibErrorNoSupportedSchemas
}

type RgbLibErrorNoValidTransportEndpoint struct {
}

func NewRgbLibErrorNoValidTransportEndpoint() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorNoValidTransportEndpoint{}}
}

func (e RgbLibErrorNoValidTransportEndpoint) destroy() {
}

func (err RgbLibErrorNoValidTransportEndpoint) Error() string {
	return fmt.Sprint("NoValidTransportEndpoint")
}

func (self RgbLibErrorNoValidTransportEndpoint) Is(target error) bool {
	return target == ErrRgbLibErrorNoValidTransportEndpoint
}

type RgbLibErrorOffline struct {
}

func NewRgbLibErrorOffline() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorOffline{}}
}

func (e RgbLibErrorOffline) destroy() {
}

func (err RgbLibErrorOffline) Error() string {
	return fmt.Sprint("Offline")
}

func (self RgbLibErrorOffline) Is(target error) bool {
	return target == ErrRgbLibErrorOffline
}

type RgbLibErrorOnlineNeeded struct {
}

func NewRgbLibErrorOnlineNeeded() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorOnlineNeeded{}}
}

func (e RgbLibErrorOnlineNeeded) destroy() {
}

func (err RgbLibErrorOnlineNeeded) Error() string {
	return fmt.Sprint("OnlineNeeded")
}

func (self RgbLibErrorOnlineNeeded) Is(target error) bool {
	return target == ErrRgbLibErrorOnlineNeeded
}

type RgbLibErrorOutputBelowDustLimit struct {
}

func NewRgbLibErrorOutputBelowDustLimit() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorOutputBelowDustLimit{}}
}

func (e RgbLibErrorOutputBelowDustLimit) destroy() {
}

func (err RgbLibErrorOutputBelowDustLimit) Error() string {
	return fmt.Sprint("OutputBelowDustLimit")
}

func (self RgbLibErrorOutputBelowDustLimit) Is(target error) bool {
	return target == ErrRgbLibErrorOutputBelowDustLimit
}

type RgbLibErrorProxy struct {
	Details string
}

func NewRgbLibErrorProxy(
	details string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorProxy{
		Details: details}}
}

func (e RgbLibErrorProxy) destroy() {
	FfiDestroyerString{}.Destroy(e.Details)
}

func (err RgbLibErrorProxy) Error() string {
	return fmt.Sprint("Proxy",
		": ",

		"Details=",
		err.Details,
	)
}

func (self RgbLibErrorProxy) Is(target error) bool {
	return target == ErrRgbLibErrorProxy
}

type RgbLibErrorRecipientIdAlreadyUsed struct {
}

func NewRgbLibErrorRecipientIdAlreadyUsed() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorRecipientIdAlreadyUsed{}}
}

func (e RgbLibErrorRecipientIdAlreadyUsed) destroy() {
}

func (err RgbLibErrorRecipientIdAlreadyUsed) Error() string {
	return fmt.Sprint("RecipientIdAlreadyUsed")
}

func (self RgbLibErrorRecipientIdAlreadyUsed) Is(target error) bool {
	return target == ErrRgbLibErrorRecipientIdAlreadyUsed
}

type RgbLibErrorRecipientIdDuplicated struct {
}

func NewRgbLibErrorRecipientIdDuplicated() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorRecipientIdDuplicated{}}
}

func (e RgbLibErrorRecipientIdDuplicated) destroy() {
}

func (err RgbLibErrorRecipientIdDuplicated) Error() string {
	return fmt.Sprint("RecipientIdDuplicated")
}

func (self RgbLibErrorRecipientIdDuplicated) Is(target error) bool {
	return target == ErrRgbLibErrorRecipientIdDuplicated
}

type RgbLibErrorTooHighInflationAmounts struct {
}

func NewRgbLibErrorTooHighInflationAmounts() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorTooHighInflationAmounts{}}
}

func (e RgbLibErrorTooHighInflationAmounts) destroy() {
}

func (err RgbLibErrorTooHighInflationAmounts) Error() string {
	return fmt.Sprint("TooHighInflationAmounts")
}

func (self RgbLibErrorTooHighInflationAmounts) Is(target error) bool {
	return target == ErrRgbLibErrorTooHighInflationAmounts
}

type RgbLibErrorTooHighIssuanceAmounts struct {
}

func NewRgbLibErrorTooHighIssuanceAmounts() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorTooHighIssuanceAmounts{}}
}

func (e RgbLibErrorTooHighIssuanceAmounts) destroy() {
}

func (err RgbLibErrorTooHighIssuanceAmounts) Error() string {
	return fmt.Sprint("TooHighIssuanceAmounts")
}

func (self RgbLibErrorTooHighIssuanceAmounts) Is(target error) bool {
	return target == ErrRgbLibErrorTooHighIssuanceAmounts
}

type RgbLibErrorUnknownRgbSchema struct {
	SchemaId string
}

func NewRgbLibErrorUnknownRgbSchema(
	schemaId string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorUnknownRgbSchema{
		SchemaId: schemaId}}
}

func (e RgbLibErrorUnknownRgbSchema) destroy() {
	FfiDestroyerString{}.Destroy(e.SchemaId)
}

func (err RgbLibErrorUnknownRgbSchema) Error() string {
	return fmt.Sprint("UnknownRgbSchema",
		": ",

		"SchemaId=",
		err.SchemaId,
	)
}

func (self RgbLibErrorUnknownRgbSchema) Is(target error) bool {
	return target == ErrRgbLibErrorUnknownRgbSchema
}

type RgbLibErrorUnsupportedBackupVersion struct {
	Version string
}

func NewRgbLibErrorUnsupportedBackupVersion(
	version string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorUnsupportedBackupVersion{
		Version: version}}
}

func (e RgbLibErrorUnsupportedBackupVersion) destroy() {
	FfiDestroyerString{}.Destroy(e.Version)
}

func (err RgbLibErrorUnsupportedBackupVersion) Error() string {
	return fmt.Sprint("UnsupportedBackupVersion",
		": ",

		"Version=",
		err.Version,
	)
}

func (self RgbLibErrorUnsupportedBackupVersion) Is(target error) bool {
	return target == ErrRgbLibErrorUnsupportedBackupVersion
}

type RgbLibErrorUnsupportedLayer1 struct {
	Layer1 string
}

func NewRgbLibErrorUnsupportedLayer1(
	layer1 string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorUnsupportedLayer1{
		Layer1: layer1}}
}

func (e RgbLibErrorUnsupportedLayer1) destroy() {
	FfiDestroyerString{}.Destroy(e.Layer1)
}

func (err RgbLibErrorUnsupportedLayer1) Error() string {
	return fmt.Sprint("UnsupportedLayer1",
		": ",

		"Layer1=",
		err.Layer1,
	)
}

func (self RgbLibErrorUnsupportedLayer1) Is(target error) bool {
	return target == ErrRgbLibErrorUnsupportedLayer1
}

type RgbLibErrorUnsupportedSchema struct {
	AssetSchema AssetSchema
}

func NewRgbLibErrorUnsupportedSchema(
	assetSchema AssetSchema,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorUnsupportedSchema{
		AssetSchema: assetSchema}}
}

func (e RgbLibErrorUnsupportedSchema) destroy() {
	FfiDestroyerAssetSchema{}.Destroy(e.AssetSchema)
}

func (err RgbLibErrorUnsupportedSchema) Error() string {
	return fmt.Sprint("UnsupportedSchema",
		": ",

		"AssetSchema=",
		err.AssetSchema,
	)
}

func (self RgbLibErrorUnsupportedSchema) Is(target error) bool {
	return target == ErrRgbLibErrorUnsupportedSchema
}

type RgbLibErrorUnsupportedTransportType struct {
}

func NewRgbLibErrorUnsupportedTransportType() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorUnsupportedTransportType{}}
}

func (e RgbLibErrorUnsupportedTransportType) destroy() {
}

func (err RgbLibErrorUnsupportedTransportType) Error() string {
	return fmt.Sprint("UnsupportedTransportType")
}

func (self RgbLibErrorUnsupportedTransportType) Is(target error) bool {
	return target == ErrRgbLibErrorUnsupportedTransportType
}

type RgbLibErrorWalletDirAlreadyExists struct {
	Path string
}

func NewRgbLibErrorWalletDirAlreadyExists(
	path string,
) *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorWalletDirAlreadyExists{
		Path: path}}
}

func (e RgbLibErrorWalletDirAlreadyExists) destroy() {
	FfiDestroyerString{}.Destroy(e.Path)
}

func (err RgbLibErrorWalletDirAlreadyExists) Error() string {
	return fmt.Sprint("WalletDirAlreadyExists",
		": ",

		"Path=",
		err.Path,
	)
}

func (self RgbLibErrorWalletDirAlreadyExists) Is(target error) bool {
	return target == ErrRgbLibErrorWalletDirAlreadyExists
}

type RgbLibErrorWatchOnly struct {
}

func NewRgbLibErrorWatchOnly() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorWatchOnly{}}
}

func (e RgbLibErrorWatchOnly) destroy() {
}

func (err RgbLibErrorWatchOnly) Error() string {
	return fmt.Sprint("WatchOnly")
}

func (self RgbLibErrorWatchOnly) Is(target error) bool {
	return target == ErrRgbLibErrorWatchOnly
}

type RgbLibErrorWrongPassword struct {
}

func NewRgbLibErrorWrongPassword() *RgbLibError {
	return &RgbLibError{err: &RgbLibErrorWrongPassword{}}
}

func (e RgbLibErrorWrongPassword) destroy() {
}

func (err RgbLibErrorWrongPassword) Error() string {
	return fmt.Sprint("WrongPassword")
}

func (self RgbLibErrorWrongPassword) Is(target error) bool {
	return target == ErrRgbLibErrorWrongPassword
}

type FfiConverterRgbLibError struct{}

var FfiConverterRgbLibErrorINSTANCE = FfiConverterRgbLibError{}

func (c FfiConverterRgbLibError) Lift(eb RustBufferI) *RgbLibError {
	return LiftFromRustBuffer[*RgbLibError](c, eb)
}

func (c FfiConverterRgbLibError) Lower(value *RgbLibError) C.RustBuffer {
	return LowerIntoRustBuffer[*RgbLibError](c, value)
}

func (c FfiConverterRgbLibError) Read(reader io.Reader) *RgbLibError {
	errorID := readUint32(reader)

	switch errorID {
	case 1:
		return &RgbLibError{&RgbLibErrorAllocationsAlreadyAvailable{}}
	case 2:
		return &RgbLibError{&RgbLibErrorAssetNotFound{
			AssetId: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 3:
		return &RgbLibError{&RgbLibErrorBatchTransferNotFound{
			Idx: FfiConverterInt32INSTANCE.Read(reader),
		}}
	case 4:
		return &RgbLibError{&RgbLibErrorBitcoinNetworkMismatch{}}
	case 5:
		return &RgbLibError{&RgbLibErrorCannotChangeOnline{}}
	case 6:
		return &RgbLibError{&RgbLibErrorCannotDeleteBatchTransfer{}}
	case 7:
		return &RgbLibError{&RgbLibErrorCannotEstimateFees{}}
	case 8:
		return &RgbLibError{&RgbLibErrorCannotFailBatchTransfer{}}
	case 9:
		return &RgbLibError{&RgbLibErrorCannotFinalizePsbt{}}
	case 10:
		return &RgbLibError{&RgbLibErrorCannotUseIfaOnMainnet{}}
	case 11:
		return &RgbLibError{&RgbLibErrorEmptyFile{
			FilePath: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 12:
		return &RgbLibError{&RgbLibErrorFailedBdkSync{
			Details: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 13:
		return &RgbLibError{&RgbLibErrorFailedBroadcast{
			Details: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 14:
		return &RgbLibError{&RgbLibErrorFailedIssuance{
			Details: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 15:
		return &RgbLibError{&RgbLibErrorFileAlreadyExists{
			Path: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 16:
		return &RgbLibError{&RgbLibErrorFingerprintMismatch{}}
	case 17:
		return &RgbLibError{&RgbLibErrorIo{
			Details: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 18:
		return &RgbLibError{&RgbLibErrorInconsistency{
			Details: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 19:
		return &RgbLibError{&RgbLibErrorIndexer{
			Details: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 20:
		return &RgbLibError{&RgbLibErrorInexistentDataDir{}}
	case 21:
		return &RgbLibError{&RgbLibErrorInsufficientAllocationSlots{}}
	case 22:
		return &RgbLibError{&RgbLibErrorInsufficientAssignments{
			AssetId:   FfiConverterStringINSTANCE.Read(reader),
			Available: FfiConverterAssignmentsCollectionINSTANCE.Read(reader),
		}}
	case 23:
		return &RgbLibError{&RgbLibErrorInsufficientBitcoins{
			Needed:    FfiConverterUint64INSTANCE.Read(reader),
			Available: FfiConverterUint64INSTANCE.Read(reader),
		}}
	case 24:
		return &RgbLibError{&RgbLibErrorInternal{
			Details: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 25:
		return &RgbLibError{&RgbLibErrorInvalidAddress{
			Details: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 26:
		return &RgbLibError{&RgbLibErrorInvalidAmountZero{}}
	case 27:
		return &RgbLibError{&RgbLibErrorInvalidAssetId{
			AssetId: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 28:
		return &RgbLibError{&RgbLibErrorInvalidAssignment{}}
	case 29:
		return &RgbLibError{&RgbLibErrorInvalidAttachments{
			Details: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 30:
		return &RgbLibError{&RgbLibErrorInvalidBitcoinKeys{}}
	case 31:
		return &RgbLibError{&RgbLibErrorInvalidBitcoinNetwork{
			Network: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 32:
		return &RgbLibError{&RgbLibErrorInvalidColoringInfo{
			Details: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 33:
		return &RgbLibError{&RgbLibErrorInvalidConsignment{}}
	case 34:
		return &RgbLibError{&RgbLibErrorInvalidDetails{
			Details: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 35:
		return &RgbLibError{&RgbLibErrorInvalidElectrum{
			Details: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 36:
		return &RgbLibError{&RgbLibErrorInvalidEstimationBlocks{}}
	case 37:
		return &RgbLibError{&RgbLibErrorInvalidFeeRate{
			Details: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 38:
		return &RgbLibError{&RgbLibErrorInvalidFilePath{
			FilePath: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 39:
		return &RgbLibError{&RgbLibErrorInvalidFingerprint{}}
	case 40:
		return &RgbLibError{&RgbLibErrorInvalidIndexer{
			Details: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 41:
		return &RgbLibError{&RgbLibErrorInvalidInvoice{
			Details: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 42:
		return &RgbLibError{&RgbLibErrorInvalidMnemonic{
			Details: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 43:
		return &RgbLibError{&RgbLibErrorInvalidName{
			Details: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 44:
		return &RgbLibError{&RgbLibErrorInvalidPrecision{
			Details: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 45:
		return &RgbLibError{&RgbLibErrorInvalidProxyProtocol{
			Version: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 46:
		return &RgbLibError{&RgbLibErrorInvalidPsbt{
			Details: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 47:
		return &RgbLibError{&RgbLibErrorInvalidPubkey{
			Details: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 48:
		return &RgbLibError{&RgbLibErrorInvalidRecipientData{
			Details: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 49:
		return &RgbLibError{&RgbLibErrorInvalidRecipientId{}}
	case 50:
		return &RgbLibError{&RgbLibErrorInvalidRecipientNetwork{}}
	case 51:
		return &RgbLibError{&RgbLibErrorInvalidTicker{
			Details: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 52:
		return &RgbLibError{&RgbLibErrorInvalidTransportEndpoint{
			Details: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 53:
		return &RgbLibError{&RgbLibErrorInvalidTransportEndpoints{
			Details: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 54:
		return &RgbLibError{&RgbLibErrorInvalidTxid{}}
	case 55:
		return &RgbLibError{&RgbLibErrorInvalidVanillaKeychain{}}
	case 56:
		return &RgbLibError{&RgbLibErrorMaxFeeExceeded{
			Txid: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 57:
		return &RgbLibError{&RgbLibErrorMinFeeNotMet{
			Txid: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 58:
		return &RgbLibError{&RgbLibErrorNetwork{
			Details: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 59:
		return &RgbLibError{&RgbLibErrorNoConsignment{}}
	case 60:
		return &RgbLibError{&RgbLibErrorNoIssuanceAmounts{}}
	case 61:
		return &RgbLibError{&RgbLibErrorNoSupportedSchemas{}}
	case 62:
		return &RgbLibError{&RgbLibErrorNoValidTransportEndpoint{}}
	case 63:
		return &RgbLibError{&RgbLibErrorOffline{}}
	case 64:
		return &RgbLibError{&RgbLibErrorOnlineNeeded{}}
	case 65:
		return &RgbLibError{&RgbLibErrorOutputBelowDustLimit{}}
	case 66:
		return &RgbLibError{&RgbLibErrorProxy{
			Details: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 67:
		return &RgbLibError{&RgbLibErrorRecipientIdAlreadyUsed{}}
	case 68:
		return &RgbLibError{&RgbLibErrorRecipientIdDuplicated{}}
	case 69:
		return &RgbLibError{&RgbLibErrorTooHighInflationAmounts{}}
	case 70:
		return &RgbLibError{&RgbLibErrorTooHighIssuanceAmounts{}}
	case 71:
		return &RgbLibError{&RgbLibErrorUnknownRgbSchema{
			SchemaId: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 72:
		return &RgbLibError{&RgbLibErrorUnsupportedBackupVersion{
			Version: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 73:
		return &RgbLibError{&RgbLibErrorUnsupportedLayer1{
			Layer1: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 74:
		return &RgbLibError{&RgbLibErrorUnsupportedSchema{
			AssetSchema: FfiConverterAssetSchemaINSTANCE.Read(reader),
		}}
	case 75:
		return &RgbLibError{&RgbLibErrorUnsupportedTransportType{}}
	case 76:
		return &RgbLibError{&RgbLibErrorWalletDirAlreadyExists{
			Path: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 77:
		return &RgbLibError{&RgbLibErrorWatchOnly{}}
	case 78:
		return &RgbLibError{&RgbLibErrorWrongPassword{}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterRgbLibError.Read()", errorID))
	}
}

func (c FfiConverterRgbLibError) Write(writer io.Writer, value *RgbLibError) {
	switch variantValue := value.err.(type) {
	case *RgbLibErrorAllocationsAlreadyAvailable:
		writeInt32(writer, 1)
	case *RgbLibErrorAssetNotFound:
		writeInt32(writer, 2)
		FfiConverterStringINSTANCE.Write(writer, variantValue.AssetId)
	case *RgbLibErrorBatchTransferNotFound:
		writeInt32(writer, 3)
		FfiConverterInt32INSTANCE.Write(writer, variantValue.Idx)
	case *RgbLibErrorBitcoinNetworkMismatch:
		writeInt32(writer, 4)
	case *RgbLibErrorCannotChangeOnline:
		writeInt32(writer, 5)
	case *RgbLibErrorCannotDeleteBatchTransfer:
		writeInt32(writer, 6)
	case *RgbLibErrorCannotEstimateFees:
		writeInt32(writer, 7)
	case *RgbLibErrorCannotFailBatchTransfer:
		writeInt32(writer, 8)
	case *RgbLibErrorCannotFinalizePsbt:
		writeInt32(writer, 9)
	case *RgbLibErrorCannotUseIfaOnMainnet:
		writeInt32(writer, 10)
	case *RgbLibErrorEmptyFile:
		writeInt32(writer, 11)
		FfiConverterStringINSTANCE.Write(writer, variantValue.FilePath)
	case *RgbLibErrorFailedBdkSync:
		writeInt32(writer, 12)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Details)
	case *RgbLibErrorFailedBroadcast:
		writeInt32(writer, 13)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Details)
	case *RgbLibErrorFailedIssuance:
		writeInt32(writer, 14)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Details)
	case *RgbLibErrorFileAlreadyExists:
		writeInt32(writer, 15)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Path)
	case *RgbLibErrorFingerprintMismatch:
		writeInt32(writer, 16)
	case *RgbLibErrorIo:
		writeInt32(writer, 17)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Details)
	case *RgbLibErrorInconsistency:
		writeInt32(writer, 18)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Details)
	case *RgbLibErrorIndexer:
		writeInt32(writer, 19)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Details)
	case *RgbLibErrorInexistentDataDir:
		writeInt32(writer, 20)
	case *RgbLibErrorInsufficientAllocationSlots:
		writeInt32(writer, 21)
	case *RgbLibErrorInsufficientAssignments:
		writeInt32(writer, 22)
		FfiConverterStringINSTANCE.Write(writer, variantValue.AssetId)
		FfiConverterAssignmentsCollectionINSTANCE.Write(writer, variantValue.Available)
	case *RgbLibErrorInsufficientBitcoins:
		writeInt32(writer, 23)
		FfiConverterUint64INSTANCE.Write(writer, variantValue.Needed)
		FfiConverterUint64INSTANCE.Write(writer, variantValue.Available)
	case *RgbLibErrorInternal:
		writeInt32(writer, 24)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Details)
	case *RgbLibErrorInvalidAddress:
		writeInt32(writer, 25)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Details)
	case *RgbLibErrorInvalidAmountZero:
		writeInt32(writer, 26)
	case *RgbLibErrorInvalidAssetId:
		writeInt32(writer, 27)
		FfiConverterStringINSTANCE.Write(writer, variantValue.AssetId)
	case *RgbLibErrorInvalidAssignment:
		writeInt32(writer, 28)
	case *RgbLibErrorInvalidAttachments:
		writeInt32(writer, 29)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Details)
	case *RgbLibErrorInvalidBitcoinKeys:
		writeInt32(writer, 30)
	case *RgbLibErrorInvalidBitcoinNetwork:
		writeInt32(writer, 31)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Network)
	case *RgbLibErrorInvalidColoringInfo:
		writeInt32(writer, 32)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Details)
	case *RgbLibErrorInvalidConsignment:
		writeInt32(writer, 33)
	case *RgbLibErrorInvalidDetails:
		writeInt32(writer, 34)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Details)
	case *RgbLibErrorInvalidElectrum:
		writeInt32(writer, 35)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Details)
	case *RgbLibErrorInvalidEstimationBlocks:
		writeInt32(writer, 36)
	case *RgbLibErrorInvalidFeeRate:
		writeInt32(writer, 37)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Details)
	case *RgbLibErrorInvalidFilePath:
		writeInt32(writer, 38)
		FfiConverterStringINSTANCE.Write(writer, variantValue.FilePath)
	case *RgbLibErrorInvalidFingerprint:
		writeInt32(writer, 39)
	case *RgbLibErrorInvalidIndexer:
		writeInt32(writer, 40)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Details)
	case *RgbLibErrorInvalidInvoice:
		writeInt32(writer, 41)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Details)
	case *RgbLibErrorInvalidMnemonic:
		writeInt32(writer, 42)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Details)
	case *RgbLibErrorInvalidName:
		writeInt32(writer, 43)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Details)
	case *RgbLibErrorInvalidPrecision:
		writeInt32(writer, 44)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Details)
	case *RgbLibErrorInvalidProxyProtocol:
		writeInt32(writer, 45)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Version)
	case *RgbLibErrorInvalidPsbt:
		writeInt32(writer, 46)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Details)
	case *RgbLibErrorInvalidPubkey:
		writeInt32(writer, 47)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Details)
	case *RgbLibErrorInvalidRecipientData:
		writeInt32(writer, 48)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Details)
	case *RgbLibErrorInvalidRecipientId:
		writeInt32(writer, 49)
	case *RgbLibErrorInvalidRecipientNetwork:
		writeInt32(writer, 50)
	case *RgbLibErrorInvalidTicker:
		writeInt32(writer, 51)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Details)
	case *RgbLibErrorInvalidTransportEndpoint:
		writeInt32(writer, 52)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Details)
	case *RgbLibErrorInvalidTransportEndpoints:
		writeInt32(writer, 53)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Details)
	case *RgbLibErrorInvalidTxid:
		writeInt32(writer, 54)
	case *RgbLibErrorInvalidVanillaKeychain:
		writeInt32(writer, 55)
	case *RgbLibErrorMaxFeeExceeded:
		writeInt32(writer, 56)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Txid)
	case *RgbLibErrorMinFeeNotMet:
		writeInt32(writer, 57)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Txid)
	case *RgbLibErrorNetwork:
		writeInt32(writer, 58)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Details)
	case *RgbLibErrorNoConsignment:
		writeInt32(writer, 59)
	case *RgbLibErrorNoIssuanceAmounts:
		writeInt32(writer, 60)
	case *RgbLibErrorNoSupportedSchemas:
		writeInt32(writer, 61)
	case *RgbLibErrorNoValidTransportEndpoint:
		writeInt32(writer, 62)
	case *RgbLibErrorOffline:
		writeInt32(writer, 63)
	case *RgbLibErrorOnlineNeeded:
		writeInt32(writer, 64)
	case *RgbLibErrorOutputBelowDustLimit:
		writeInt32(writer, 65)
	case *RgbLibErrorProxy:
		writeInt32(writer, 66)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Details)
	case *RgbLibErrorRecipientIdAlreadyUsed:
		writeInt32(writer, 67)
	case *RgbLibErrorRecipientIdDuplicated:
		writeInt32(writer, 68)
	case *RgbLibErrorTooHighInflationAmounts:
		writeInt32(writer, 69)
	case *RgbLibErrorTooHighIssuanceAmounts:
		writeInt32(writer, 70)
	case *RgbLibErrorUnknownRgbSchema:
		writeInt32(writer, 71)
		FfiConverterStringINSTANCE.Write(writer, variantValue.SchemaId)
	case *RgbLibErrorUnsupportedBackupVersion:
		writeInt32(writer, 72)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Version)
	case *RgbLibErrorUnsupportedLayer1:
		writeInt32(writer, 73)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Layer1)
	case *RgbLibErrorUnsupportedSchema:
		writeInt32(writer, 74)
		FfiConverterAssetSchemaINSTANCE.Write(writer, variantValue.AssetSchema)
	case *RgbLibErrorUnsupportedTransportType:
		writeInt32(writer, 75)
	case *RgbLibErrorWalletDirAlreadyExists:
		writeInt32(writer, 76)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Path)
	case *RgbLibErrorWatchOnly:
		writeInt32(writer, 77)
	case *RgbLibErrorWrongPassword:
		writeInt32(writer, 78)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterRgbLibError.Write", value))
	}
}

type FfiDestroyerRgbLibError struct{}

func (_ FfiDestroyerRgbLibError) Destroy(value *RgbLibError) {
	switch variantValue := value.err.(type) {
	case RgbLibErrorAllocationsAlreadyAvailable:
		variantValue.destroy()
	case RgbLibErrorAssetNotFound:
		variantValue.destroy()
	case RgbLibErrorBatchTransferNotFound:
		variantValue.destroy()
	case RgbLibErrorBitcoinNetworkMismatch:
		variantValue.destroy()
	case RgbLibErrorCannotChangeOnline:
		variantValue.destroy()
	case RgbLibErrorCannotDeleteBatchTransfer:
		variantValue.destroy()
	case RgbLibErrorCannotEstimateFees:
		variantValue.destroy()
	case RgbLibErrorCannotFailBatchTransfer:
		variantValue.destroy()
	case RgbLibErrorCannotFinalizePsbt:
		variantValue.destroy()
	case RgbLibErrorCannotUseIfaOnMainnet:
		variantValue.destroy()
	case RgbLibErrorEmptyFile:
		variantValue.destroy()
	case RgbLibErrorFailedBdkSync:
		variantValue.destroy()
	case RgbLibErrorFailedBroadcast:
		variantValue.destroy()
	case RgbLibErrorFailedIssuance:
		variantValue.destroy()
	case RgbLibErrorFileAlreadyExists:
		variantValue.destroy()
	case RgbLibErrorFingerprintMismatch:
		variantValue.destroy()
	case RgbLibErrorIo:
		variantValue.destroy()
	case RgbLibErrorInconsistency:
		variantValue.destroy()
	case RgbLibErrorIndexer:
		variantValue.destroy()
	case RgbLibErrorInexistentDataDir:
		variantValue.destroy()
	case RgbLibErrorInsufficientAllocationSlots:
		variantValue.destroy()
	case RgbLibErrorInsufficientAssignments:
		variantValue.destroy()
	case RgbLibErrorInsufficientBitcoins:
		variantValue.destroy()
	case RgbLibErrorInternal:
		variantValue.destroy()
	case RgbLibErrorInvalidAddress:
		variantValue.destroy()
	case RgbLibErrorInvalidAmountZero:
		variantValue.destroy()
	case RgbLibErrorInvalidAssetId:
		variantValue.destroy()
	case RgbLibErrorInvalidAssignment:
		variantValue.destroy()
	case RgbLibErrorInvalidAttachments:
		variantValue.destroy()
	case RgbLibErrorInvalidBitcoinKeys:
		variantValue.destroy()
	case RgbLibErrorInvalidBitcoinNetwork:
		variantValue.destroy()
	case RgbLibErrorInvalidColoringInfo:
		variantValue.destroy()
	case RgbLibErrorInvalidConsignment:
		variantValue.destroy()
	case RgbLibErrorInvalidDetails:
		variantValue.destroy()
	case RgbLibErrorInvalidElectrum:
		variantValue.destroy()
	case RgbLibErrorInvalidEstimationBlocks:
		variantValue.destroy()
	case RgbLibErrorInvalidFeeRate:
		variantValue.destroy()
	case RgbLibErrorInvalidFilePath:
		variantValue.destroy()
	case RgbLibErrorInvalidFingerprint:
		variantValue.destroy()
	case RgbLibErrorInvalidIndexer:
		variantValue.destroy()
	case RgbLibErrorInvalidInvoice:
		variantValue.destroy()
	case RgbLibErrorInvalidMnemonic:
		variantValue.destroy()
	case RgbLibErrorInvalidName:
		variantValue.destroy()
	case RgbLibErrorInvalidPrecision:
		variantValue.destroy()
	case RgbLibErrorInvalidProxyProtocol:
		variantValue.destroy()
	case RgbLibErrorInvalidPsbt:
		variantValue.destroy()
	case RgbLibErrorInvalidPubkey:
		variantValue.destroy()
	case RgbLibErrorInvalidRecipientData:
		variantValue.destroy()
	case RgbLibErrorInvalidRecipientId:
		variantValue.destroy()
	case RgbLibErrorInvalidRecipientNetwork:
		variantValue.destroy()
	case RgbLibErrorInvalidTicker:
		variantValue.destroy()
	case RgbLibErrorInvalidTransportEndpoint:
		variantValue.destroy()
	case RgbLibErrorInvalidTransportEndpoints:
		variantValue.destroy()
	case RgbLibErrorInvalidTxid:
		variantValue.destroy()
	case RgbLibErrorInvalidVanillaKeychain:
		variantValue.destroy()
	case RgbLibErrorMaxFeeExceeded:
		variantValue.destroy()
	case RgbLibErrorMinFeeNotMet:
		variantValue.destroy()
	case RgbLibErrorNetwork:
		variantValue.destroy()
	case RgbLibErrorNoConsignment:
		variantValue.destroy()
	case RgbLibErrorNoIssuanceAmounts:
		variantValue.destroy()
	case RgbLibErrorNoSupportedSchemas:
		variantValue.destroy()
	case RgbLibErrorNoValidTransportEndpoint:
		variantValue.destroy()
	case RgbLibErrorOffline:
		variantValue.destroy()
	case RgbLibErrorOnlineNeeded:
		variantValue.destroy()
	case RgbLibErrorOutputBelowDustLimit:
		variantValue.destroy()
	case RgbLibErrorProxy:
		variantValue.destroy()
	case RgbLibErrorRecipientIdAlreadyUsed:
		variantValue.destroy()
	case RgbLibErrorRecipientIdDuplicated:
		variantValue.destroy()
	case RgbLibErrorTooHighInflationAmounts:
		variantValue.destroy()
	case RgbLibErrorTooHighIssuanceAmounts:
		variantValue.destroy()
	case RgbLibErrorUnknownRgbSchema:
		variantValue.destroy()
	case RgbLibErrorUnsupportedBackupVersion:
		variantValue.destroy()
	case RgbLibErrorUnsupportedLayer1:
		variantValue.destroy()
	case RgbLibErrorUnsupportedSchema:
		variantValue.destroy()
	case RgbLibErrorUnsupportedTransportType:
		variantValue.destroy()
	case RgbLibErrorWalletDirAlreadyExists:
		variantValue.destroy()
	case RgbLibErrorWatchOnly:
		variantValue.destroy()
	case RgbLibErrorWrongPassword:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerRgbLibError.Destroy", value))
	}
}

type TransactionType uint

const (
	TransactionTypeRgbSend     TransactionType = 1
	TransactionTypeDrain       TransactionType = 2
	TransactionTypeCreateUtxos TransactionType = 3
	TransactionTypeUser        TransactionType = 4
)

type FfiConverterTransactionType struct{}

var FfiConverterTransactionTypeINSTANCE = FfiConverterTransactionType{}

func (c FfiConverterTransactionType) Lift(rb RustBufferI) TransactionType {
	return LiftFromRustBuffer[TransactionType](c, rb)
}

func (c FfiConverterTransactionType) Lower(value TransactionType) C.RustBuffer {
	return LowerIntoRustBuffer[TransactionType](c, value)
}
func (FfiConverterTransactionType) Read(reader io.Reader) TransactionType {
	id := readInt32(reader)
	return TransactionType(id)
}

func (FfiConverterTransactionType) Write(writer io.Writer, value TransactionType) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerTransactionType struct{}

func (_ FfiDestroyerTransactionType) Destroy(value TransactionType) {
}

type TransferKind uint

const (
	TransferKindIssuance       TransferKind = 1
	TransferKindReceiveBlind   TransferKind = 2
	TransferKindReceiveWitness TransferKind = 3
	TransferKindSend           TransferKind = 4
)

type FfiConverterTransferKind struct{}

var FfiConverterTransferKindINSTANCE = FfiConverterTransferKind{}

func (c FfiConverterTransferKind) Lift(rb RustBufferI) TransferKind {
	return LiftFromRustBuffer[TransferKind](c, rb)
}

func (c FfiConverterTransferKind) Lower(value TransferKind) C.RustBuffer {
	return LowerIntoRustBuffer[TransferKind](c, value)
}
func (FfiConverterTransferKind) Read(reader io.Reader) TransferKind {
	id := readInt32(reader)
	return TransferKind(id)
}

func (FfiConverterTransferKind) Write(writer io.Writer, value TransferKind) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerTransferKind struct{}

func (_ FfiDestroyerTransferKind) Destroy(value TransferKind) {
}

type TransferStatus uint

const (
	TransferStatusWaitingCounterparty  TransferStatus = 1
	TransferStatusWaitingConfirmations TransferStatus = 2
	TransferStatusSettled              TransferStatus = 3
	TransferStatusFailed               TransferStatus = 4
)

type FfiConverterTransferStatus struct{}

var FfiConverterTransferStatusINSTANCE = FfiConverterTransferStatus{}

func (c FfiConverterTransferStatus) Lift(rb RustBufferI) TransferStatus {
	return LiftFromRustBuffer[TransferStatus](c, rb)
}

func (c FfiConverterTransferStatus) Lower(value TransferStatus) C.RustBuffer {
	return LowerIntoRustBuffer[TransferStatus](c, value)
}
func (FfiConverterTransferStatus) Read(reader io.Reader) TransferStatus {
	id := readInt32(reader)
	return TransferStatus(id)
}

func (FfiConverterTransferStatus) Write(writer io.Writer, value TransferStatus) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerTransferStatus struct{}

func (_ FfiDestroyerTransferStatus) Destroy(value TransferStatus) {
}

type TransportType uint

const (
	TransportTypeJsonRpc TransportType = 1
)

type FfiConverterTransportType struct{}

var FfiConverterTransportTypeINSTANCE = FfiConverterTransportType{}

func (c FfiConverterTransportType) Lift(rb RustBufferI) TransportType {
	return LiftFromRustBuffer[TransportType](c, rb)
}

func (c FfiConverterTransportType) Lower(value TransportType) C.RustBuffer {
	return LowerIntoRustBuffer[TransportType](c, value)
}
func (FfiConverterTransportType) Read(reader io.Reader) TransportType {
	id := readInt32(reader)
	return TransportType(id)
}

func (FfiConverterTransportType) Write(writer io.Writer, value TransportType) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerTransportType struct{}

func (_ FfiDestroyerTransportType) Destroy(value TransportType) {
}

type FfiConverterOptionalUint8 struct{}

var FfiConverterOptionalUint8INSTANCE = FfiConverterOptionalUint8{}

func (c FfiConverterOptionalUint8) Lift(rb RustBufferI) *uint8 {
	return LiftFromRustBuffer[*uint8](c, rb)
}

func (_ FfiConverterOptionalUint8) Read(reader io.Reader) *uint8 {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterUint8INSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalUint8) Lower(value *uint8) C.RustBuffer {
	return LowerIntoRustBuffer[*uint8](c, value)
}

func (_ FfiConverterOptionalUint8) Write(writer io.Writer, value *uint8) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterUint8INSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalUint8 struct{}

func (_ FfiDestroyerOptionalUint8) Destroy(value *uint8) {
	if value != nil {
		FfiDestroyerUint8{}.Destroy(*value)
	}
}

type FfiConverterOptionalUint32 struct{}

var FfiConverterOptionalUint32INSTANCE = FfiConverterOptionalUint32{}

func (c FfiConverterOptionalUint32) Lift(rb RustBufferI) *uint32 {
	return LiftFromRustBuffer[*uint32](c, rb)
}

func (_ FfiConverterOptionalUint32) Read(reader io.Reader) *uint32 {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterUint32INSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalUint32) Lower(value *uint32) C.RustBuffer {
	return LowerIntoRustBuffer[*uint32](c, value)
}

func (_ FfiConverterOptionalUint32) Write(writer io.Writer, value *uint32) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterUint32INSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalUint32 struct{}

func (_ FfiDestroyerOptionalUint32) Destroy(value *uint32) {
	if value != nil {
		FfiDestroyerUint32{}.Destroy(*value)
	}
}

type FfiConverterOptionalInt32 struct{}

var FfiConverterOptionalInt32INSTANCE = FfiConverterOptionalInt32{}

func (c FfiConverterOptionalInt32) Lift(rb RustBufferI) *int32 {
	return LiftFromRustBuffer[*int32](c, rb)
}

func (_ FfiConverterOptionalInt32) Read(reader io.Reader) *int32 {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterInt32INSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalInt32) Lower(value *int32) C.RustBuffer {
	return LowerIntoRustBuffer[*int32](c, value)
}

func (_ FfiConverterOptionalInt32) Write(writer io.Writer, value *int32) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterInt32INSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalInt32 struct{}

func (_ FfiDestroyerOptionalInt32) Destroy(value *int32) {
	if value != nil {
		FfiDestroyerInt32{}.Destroy(*value)
	}
}

type FfiConverterOptionalUint64 struct{}

var FfiConverterOptionalUint64INSTANCE = FfiConverterOptionalUint64{}

func (c FfiConverterOptionalUint64) Lift(rb RustBufferI) *uint64 {
	return LiftFromRustBuffer[*uint64](c, rb)
}

func (_ FfiConverterOptionalUint64) Read(reader io.Reader) *uint64 {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterUint64INSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalUint64) Lower(value *uint64) C.RustBuffer {
	return LowerIntoRustBuffer[*uint64](c, value)
}

func (_ FfiConverterOptionalUint64) Write(writer io.Writer, value *uint64) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterUint64INSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalUint64 struct{}

func (_ FfiDestroyerOptionalUint64) Destroy(value *uint64) {
	if value != nil {
		FfiDestroyerUint64{}.Destroy(*value)
	}
}

type FfiConverterOptionalInt64 struct{}

var FfiConverterOptionalInt64INSTANCE = FfiConverterOptionalInt64{}

func (c FfiConverterOptionalInt64) Lift(rb RustBufferI) *int64 {
	return LiftFromRustBuffer[*int64](c, rb)
}

func (_ FfiConverterOptionalInt64) Read(reader io.Reader) *int64 {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterInt64INSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalInt64) Lower(value *int64) C.RustBuffer {
	return LowerIntoRustBuffer[*int64](c, value)
}

func (_ FfiConverterOptionalInt64) Write(writer io.Writer, value *int64) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterInt64INSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalInt64 struct{}

func (_ FfiDestroyerOptionalInt64) Destroy(value *int64) {
	if value != nil {
		FfiDestroyerInt64{}.Destroy(*value)
	}
}

type FfiConverterOptionalString struct{}

var FfiConverterOptionalStringINSTANCE = FfiConverterOptionalString{}

func (c FfiConverterOptionalString) Lift(rb RustBufferI) *string {
	return LiftFromRustBuffer[*string](c, rb)
}

func (_ FfiConverterOptionalString) Read(reader io.Reader) *string {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterStringINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalString) Lower(value *string) C.RustBuffer {
	return LowerIntoRustBuffer[*string](c, value)
}

func (_ FfiConverterOptionalString) Write(writer io.Writer, value *string) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterStringINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalString struct{}

func (_ FfiDestroyerOptionalString) Destroy(value *string) {
	if value != nil {
		FfiDestroyerString{}.Destroy(*value)
	}
}

type FfiConverterOptionalBlockTime struct{}

var FfiConverterOptionalBlockTimeINSTANCE = FfiConverterOptionalBlockTime{}

func (c FfiConverterOptionalBlockTime) Lift(rb RustBufferI) *BlockTime {
	return LiftFromRustBuffer[*BlockTime](c, rb)
}

func (_ FfiConverterOptionalBlockTime) Read(reader io.Reader) *BlockTime {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterBlockTimeINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalBlockTime) Lower(value *BlockTime) C.RustBuffer {
	return LowerIntoRustBuffer[*BlockTime](c, value)
}

func (_ FfiConverterOptionalBlockTime) Write(writer io.Writer, value *BlockTime) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterBlockTimeINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalBlockTime struct{}

func (_ FfiDestroyerOptionalBlockTime) Destroy(value *BlockTime) {
	if value != nil {
		FfiDestroyerBlockTime{}.Destroy(*value)
	}
}

type FfiConverterOptionalEmbeddedMedia struct{}

var FfiConverterOptionalEmbeddedMediaINSTANCE = FfiConverterOptionalEmbeddedMedia{}

func (c FfiConverterOptionalEmbeddedMedia) Lift(rb RustBufferI) *EmbeddedMedia {
	return LiftFromRustBuffer[*EmbeddedMedia](c, rb)
}

func (_ FfiConverterOptionalEmbeddedMedia) Read(reader io.Reader) *EmbeddedMedia {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterEmbeddedMediaINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalEmbeddedMedia) Lower(value *EmbeddedMedia) C.RustBuffer {
	return LowerIntoRustBuffer[*EmbeddedMedia](c, value)
}

func (_ FfiConverterOptionalEmbeddedMedia) Write(writer io.Writer, value *EmbeddedMedia) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterEmbeddedMediaINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalEmbeddedMedia struct{}

func (_ FfiDestroyerOptionalEmbeddedMedia) Destroy(value *EmbeddedMedia) {
	if value != nil {
		FfiDestroyerEmbeddedMedia{}.Destroy(*value)
	}
}

type FfiConverterOptionalMedia struct{}

var FfiConverterOptionalMediaINSTANCE = FfiConverterOptionalMedia{}

func (c FfiConverterOptionalMedia) Lift(rb RustBufferI) *Media {
	return LiftFromRustBuffer[*Media](c, rb)
}

func (_ FfiConverterOptionalMedia) Read(reader io.Reader) *Media {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterMediaINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalMedia) Lower(value *Media) C.RustBuffer {
	return LowerIntoRustBuffer[*Media](c, value)
}

func (_ FfiConverterOptionalMedia) Write(writer io.Writer, value *Media) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterMediaINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalMedia struct{}

func (_ FfiDestroyerOptionalMedia) Destroy(value *Media) {
	if value != nil {
		FfiDestroyerMedia{}.Destroy(*value)
	}
}

type FfiConverterOptionalOnline struct{}

var FfiConverterOptionalOnlineINSTANCE = FfiConverterOptionalOnline{}

func (c FfiConverterOptionalOnline) Lift(rb RustBufferI) *Online {
	return LiftFromRustBuffer[*Online](c, rb)
}

func (_ FfiConverterOptionalOnline) Read(reader io.Reader) *Online {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterOnlineINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalOnline) Lower(value *Online) C.RustBuffer {
	return LowerIntoRustBuffer[*Online](c, value)
}

func (_ FfiConverterOptionalOnline) Write(writer io.Writer, value *Online) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterOnlineINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalOnline struct{}

func (_ FfiDestroyerOptionalOnline) Destroy(value *Online) {
	if value != nil {
		FfiDestroyerOnline{}.Destroy(*value)
	}
}

type FfiConverterOptionalOutpoint struct{}

var FfiConverterOptionalOutpointINSTANCE = FfiConverterOptionalOutpoint{}

func (c FfiConverterOptionalOutpoint) Lift(rb RustBufferI) *Outpoint {
	return LiftFromRustBuffer[*Outpoint](c, rb)
}

func (_ FfiConverterOptionalOutpoint) Read(reader io.Reader) *Outpoint {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterOutpointINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalOutpoint) Lower(value *Outpoint) C.RustBuffer {
	return LowerIntoRustBuffer[*Outpoint](c, value)
}

func (_ FfiConverterOptionalOutpoint) Write(writer io.Writer, value *Outpoint) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterOutpointINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalOutpoint struct{}

func (_ FfiDestroyerOptionalOutpoint) Destroy(value *Outpoint) {
	if value != nil {
		FfiDestroyerOutpoint{}.Destroy(*value)
	}
}

type FfiConverterOptionalProofOfReserves struct{}

var FfiConverterOptionalProofOfReservesINSTANCE = FfiConverterOptionalProofOfReserves{}

func (c FfiConverterOptionalProofOfReserves) Lift(rb RustBufferI) *ProofOfReserves {
	return LiftFromRustBuffer[*ProofOfReserves](c, rb)
}

func (_ FfiConverterOptionalProofOfReserves) Read(reader io.Reader) *ProofOfReserves {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterProofOfReservesINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalProofOfReserves) Lower(value *ProofOfReserves) C.RustBuffer {
	return LowerIntoRustBuffer[*ProofOfReserves](c, value)
}

func (_ FfiConverterOptionalProofOfReserves) Write(writer io.Writer, value *ProofOfReserves) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterProofOfReservesINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalProofOfReserves struct{}

func (_ FfiDestroyerOptionalProofOfReserves) Destroy(value *ProofOfReserves) {
	if value != nil {
		FfiDestroyerProofOfReserves{}.Destroy(*value)
	}
}

type FfiConverterOptionalToken struct{}

var FfiConverterOptionalTokenINSTANCE = FfiConverterOptionalToken{}

func (c FfiConverterOptionalToken) Lift(rb RustBufferI) *Token {
	return LiftFromRustBuffer[*Token](c, rb)
}

func (_ FfiConverterOptionalToken) Read(reader io.Reader) *Token {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterTokenINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalToken) Lower(value *Token) C.RustBuffer {
	return LowerIntoRustBuffer[*Token](c, value)
}

func (_ FfiConverterOptionalToken) Write(writer io.Writer, value *Token) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterTokenINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalToken struct{}

func (_ FfiDestroyerOptionalToken) Destroy(value *Token) {
	if value != nil {
		FfiDestroyerToken{}.Destroy(*value)
	}
}

type FfiConverterOptionalTokenLight struct{}

var FfiConverterOptionalTokenLightINSTANCE = FfiConverterOptionalTokenLight{}

func (c FfiConverterOptionalTokenLight) Lift(rb RustBufferI) *TokenLight {
	return LiftFromRustBuffer[*TokenLight](c, rb)
}

func (_ FfiConverterOptionalTokenLight) Read(reader io.Reader) *TokenLight {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterTokenLightINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalTokenLight) Lower(value *TokenLight) C.RustBuffer {
	return LowerIntoRustBuffer[*TokenLight](c, value)
}

func (_ FfiConverterOptionalTokenLight) Write(writer io.Writer, value *TokenLight) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterTokenLightINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalTokenLight struct{}

func (_ FfiDestroyerOptionalTokenLight) Destroy(value *TokenLight) {
	if value != nil {
		FfiDestroyerTokenLight{}.Destroy(*value)
	}
}

type FfiConverterOptionalWitnessData struct{}

var FfiConverterOptionalWitnessDataINSTANCE = FfiConverterOptionalWitnessData{}

func (c FfiConverterOptionalWitnessData) Lift(rb RustBufferI) *WitnessData {
	return LiftFromRustBuffer[*WitnessData](c, rb)
}

func (_ FfiConverterOptionalWitnessData) Read(reader io.Reader) *WitnessData {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterWitnessDataINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalWitnessData) Lower(value *WitnessData) C.RustBuffer {
	return LowerIntoRustBuffer[*WitnessData](c, value)
}

func (_ FfiConverterOptionalWitnessData) Write(writer io.Writer, value *WitnessData) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterWitnessDataINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalWitnessData struct{}

func (_ FfiDestroyerOptionalWitnessData) Destroy(value *WitnessData) {
	if value != nil {
		FfiDestroyerWitnessData{}.Destroy(*value)
	}
}

type FfiConverterOptionalAssetSchema struct{}

var FfiConverterOptionalAssetSchemaINSTANCE = FfiConverterOptionalAssetSchema{}

func (c FfiConverterOptionalAssetSchema) Lift(rb RustBufferI) *AssetSchema {
	return LiftFromRustBuffer[*AssetSchema](c, rb)
}

func (_ FfiConverterOptionalAssetSchema) Read(reader io.Reader) *AssetSchema {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterAssetSchemaINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalAssetSchema) Lower(value *AssetSchema) C.RustBuffer {
	return LowerIntoRustBuffer[*AssetSchema](c, value)
}

func (_ FfiConverterOptionalAssetSchema) Write(writer io.Writer, value *AssetSchema) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterAssetSchemaINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalAssetSchema struct{}

func (_ FfiDestroyerOptionalAssetSchema) Destroy(value *AssetSchema) {
	if value != nil {
		FfiDestroyerAssetSchema{}.Destroy(*value)
	}
}

type FfiConverterOptionalAssignment struct{}

var FfiConverterOptionalAssignmentINSTANCE = FfiConverterOptionalAssignment{}

func (c FfiConverterOptionalAssignment) Lift(rb RustBufferI) *Assignment {
	return LiftFromRustBuffer[*Assignment](c, rb)
}

func (_ FfiConverterOptionalAssignment) Read(reader io.Reader) *Assignment {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterAssignmentINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalAssignment) Lower(value *Assignment) C.RustBuffer {
	return LowerIntoRustBuffer[*Assignment](c, value)
}

func (_ FfiConverterOptionalAssignment) Write(writer io.Writer, value *Assignment) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterAssignmentINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalAssignment struct{}

func (_ FfiDestroyerOptionalAssignment) Destroy(value *Assignment) {
	if value != nil {
		FfiDestroyerAssignment{}.Destroy(*value)
	}
}

type FfiConverterOptionalRgbLibError struct{}

var FfiConverterOptionalRgbLibErrorINSTANCE = FfiConverterOptionalRgbLibError{}

func (c FfiConverterOptionalRgbLibError) Lift(rb RustBufferI) **RgbLibError {
	return LiftFromRustBuffer[**RgbLibError](c, rb)
}

func (_ FfiConverterOptionalRgbLibError) Read(reader io.Reader) **RgbLibError {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterRgbLibErrorINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalRgbLibError) Lower(value **RgbLibError) C.RustBuffer {
	return LowerIntoRustBuffer[**RgbLibError](c, value)
}

func (_ FfiConverterOptionalRgbLibError) Write(writer io.Writer, value **RgbLibError) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterRgbLibErrorINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalRgbLibError struct{}

func (_ FfiDestroyerOptionalRgbLibError) Destroy(value **RgbLibError) {
	if value != nil {
		FfiDestroyerRgbLibError{}.Destroy(*value)
	}
}

type FfiConverterOptionalTransferStatus struct{}

var FfiConverterOptionalTransferStatusINSTANCE = FfiConverterOptionalTransferStatus{}

func (c FfiConverterOptionalTransferStatus) Lift(rb RustBufferI) *TransferStatus {
	return LiftFromRustBuffer[*TransferStatus](c, rb)
}

func (_ FfiConverterOptionalTransferStatus) Read(reader io.Reader) *TransferStatus {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterTransferStatusINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalTransferStatus) Lower(value *TransferStatus) C.RustBuffer {
	return LowerIntoRustBuffer[*TransferStatus](c, value)
}

func (_ FfiConverterOptionalTransferStatus) Write(writer io.Writer, value *TransferStatus) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterTransferStatusINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalTransferStatus struct{}

func (_ FfiDestroyerOptionalTransferStatus) Destroy(value *TransferStatus) {
	if value != nil {
		FfiDestroyerTransferStatus{}.Destroy(*value)
	}
}

type FfiConverterOptionalSequenceAssetCfa struct{}

var FfiConverterOptionalSequenceAssetCfaINSTANCE = FfiConverterOptionalSequenceAssetCfa{}

func (c FfiConverterOptionalSequenceAssetCfa) Lift(rb RustBufferI) *[]AssetCfa {
	return LiftFromRustBuffer[*[]AssetCfa](c, rb)
}

func (_ FfiConverterOptionalSequenceAssetCfa) Read(reader io.Reader) *[]AssetCfa {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterSequenceAssetCfaINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalSequenceAssetCfa) Lower(value *[]AssetCfa) C.RustBuffer {
	return LowerIntoRustBuffer[*[]AssetCfa](c, value)
}

func (_ FfiConverterOptionalSequenceAssetCfa) Write(writer io.Writer, value *[]AssetCfa) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterSequenceAssetCfaINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalSequenceAssetCfa struct{}

func (_ FfiDestroyerOptionalSequenceAssetCfa) Destroy(value *[]AssetCfa) {
	if value != nil {
		FfiDestroyerSequenceAssetCfa{}.Destroy(*value)
	}
}

type FfiConverterOptionalSequenceAssetIfa struct{}

var FfiConverterOptionalSequenceAssetIfaINSTANCE = FfiConverterOptionalSequenceAssetIfa{}

func (c FfiConverterOptionalSequenceAssetIfa) Lift(rb RustBufferI) *[]AssetIfa {
	return LiftFromRustBuffer[*[]AssetIfa](c, rb)
}

func (_ FfiConverterOptionalSequenceAssetIfa) Read(reader io.Reader) *[]AssetIfa {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterSequenceAssetIfaINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalSequenceAssetIfa) Lower(value *[]AssetIfa) C.RustBuffer {
	return LowerIntoRustBuffer[*[]AssetIfa](c, value)
}

func (_ FfiConverterOptionalSequenceAssetIfa) Write(writer io.Writer, value *[]AssetIfa) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterSequenceAssetIfaINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalSequenceAssetIfa struct{}

func (_ FfiDestroyerOptionalSequenceAssetIfa) Destroy(value *[]AssetIfa) {
	if value != nil {
		FfiDestroyerSequenceAssetIfa{}.Destroy(*value)
	}
}

type FfiConverterOptionalSequenceAssetNia struct{}

var FfiConverterOptionalSequenceAssetNiaINSTANCE = FfiConverterOptionalSequenceAssetNia{}

func (c FfiConverterOptionalSequenceAssetNia) Lift(rb RustBufferI) *[]AssetNia {
	return LiftFromRustBuffer[*[]AssetNia](c, rb)
}

func (_ FfiConverterOptionalSequenceAssetNia) Read(reader io.Reader) *[]AssetNia {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterSequenceAssetNiaINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalSequenceAssetNia) Lower(value *[]AssetNia) C.RustBuffer {
	return LowerIntoRustBuffer[*[]AssetNia](c, value)
}

func (_ FfiConverterOptionalSequenceAssetNia) Write(writer io.Writer, value *[]AssetNia) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterSequenceAssetNiaINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalSequenceAssetNia struct{}

func (_ FfiDestroyerOptionalSequenceAssetNia) Destroy(value *[]AssetNia) {
	if value != nil {
		FfiDestroyerSequenceAssetNia{}.Destroy(*value)
	}
}

type FfiConverterOptionalSequenceAssetUda struct{}

var FfiConverterOptionalSequenceAssetUdaINSTANCE = FfiConverterOptionalSequenceAssetUda{}

func (c FfiConverterOptionalSequenceAssetUda) Lift(rb RustBufferI) *[]AssetUda {
	return LiftFromRustBuffer[*[]AssetUda](c, rb)
}

func (_ FfiConverterOptionalSequenceAssetUda) Read(reader io.Reader) *[]AssetUda {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterSequenceAssetUdaINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalSequenceAssetUda) Lower(value *[]AssetUda) C.RustBuffer {
	return LowerIntoRustBuffer[*[]AssetUda](c, value)
}

func (_ FfiConverterOptionalSequenceAssetUda) Write(writer io.Writer, value *[]AssetUda) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterSequenceAssetUdaINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalSequenceAssetUda struct{}

func (_ FfiDestroyerOptionalSequenceAssetUda) Destroy(value *[]AssetUda) {
	if value != nil {
		FfiDestroyerSequenceAssetUda{}.Destroy(*value)
	}
}

type FfiConverterSequenceUint8 struct{}

var FfiConverterSequenceUint8INSTANCE = FfiConverterSequenceUint8{}

func (c FfiConverterSequenceUint8) Lift(rb RustBufferI) []uint8 {
	return LiftFromRustBuffer[[]uint8](c, rb)
}

func (c FfiConverterSequenceUint8) Read(reader io.Reader) []uint8 {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]uint8, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterUint8INSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceUint8) Lower(value []uint8) C.RustBuffer {
	return LowerIntoRustBuffer[[]uint8](c, value)
}

func (c FfiConverterSequenceUint8) Write(writer io.Writer, value []uint8) {
	if len(value) > math.MaxInt32 {
		panic("[]uint8 is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterUint8INSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceUint8 struct{}

func (FfiDestroyerSequenceUint8) Destroy(sequence []uint8) {
	for _, value := range sequence {
		FfiDestroyerUint8{}.Destroy(value)
	}
}

type FfiConverterSequenceUint64 struct{}

var FfiConverterSequenceUint64INSTANCE = FfiConverterSequenceUint64{}

func (c FfiConverterSequenceUint64) Lift(rb RustBufferI) []uint64 {
	return LiftFromRustBuffer[[]uint64](c, rb)
}

func (c FfiConverterSequenceUint64) Read(reader io.Reader) []uint64 {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]uint64, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterUint64INSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceUint64) Lower(value []uint64) C.RustBuffer {
	return LowerIntoRustBuffer[[]uint64](c, value)
}

func (c FfiConverterSequenceUint64) Write(writer io.Writer, value []uint64) {
	if len(value) > math.MaxInt32 {
		panic("[]uint64 is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterUint64INSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceUint64 struct{}

func (FfiDestroyerSequenceUint64) Destroy(sequence []uint64) {
	for _, value := range sequence {
		FfiDestroyerUint64{}.Destroy(value)
	}
}

type FfiConverterSequenceString struct{}

var FfiConverterSequenceStringINSTANCE = FfiConverterSequenceString{}

func (c FfiConverterSequenceString) Lift(rb RustBufferI) []string {
	return LiftFromRustBuffer[[]string](c, rb)
}

func (c FfiConverterSequenceString) Read(reader io.Reader) []string {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]string, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterStringINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceString) Lower(value []string) C.RustBuffer {
	return LowerIntoRustBuffer[[]string](c, value)
}

func (c FfiConverterSequenceString) Write(writer io.Writer, value []string) {
	if len(value) > math.MaxInt32 {
		panic("[]string is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterStringINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceString struct{}

func (FfiDestroyerSequenceString) Destroy(sequence []string) {
	for _, value := range sequence {
		FfiDestroyerString{}.Destroy(value)
	}
}

type FfiConverterSequenceAssetCfa struct{}

var FfiConverterSequenceAssetCfaINSTANCE = FfiConverterSequenceAssetCfa{}

func (c FfiConverterSequenceAssetCfa) Lift(rb RustBufferI) []AssetCfa {
	return LiftFromRustBuffer[[]AssetCfa](c, rb)
}

func (c FfiConverterSequenceAssetCfa) Read(reader io.Reader) []AssetCfa {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]AssetCfa, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterAssetCfaINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceAssetCfa) Lower(value []AssetCfa) C.RustBuffer {
	return LowerIntoRustBuffer[[]AssetCfa](c, value)
}

func (c FfiConverterSequenceAssetCfa) Write(writer io.Writer, value []AssetCfa) {
	if len(value) > math.MaxInt32 {
		panic("[]AssetCfa is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterAssetCfaINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceAssetCfa struct{}

func (FfiDestroyerSequenceAssetCfa) Destroy(sequence []AssetCfa) {
	for _, value := range sequence {
		FfiDestroyerAssetCfa{}.Destroy(value)
	}
}

type FfiConverterSequenceAssetIfa struct{}

var FfiConverterSequenceAssetIfaINSTANCE = FfiConverterSequenceAssetIfa{}

func (c FfiConverterSequenceAssetIfa) Lift(rb RustBufferI) []AssetIfa {
	return LiftFromRustBuffer[[]AssetIfa](c, rb)
}

func (c FfiConverterSequenceAssetIfa) Read(reader io.Reader) []AssetIfa {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]AssetIfa, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterAssetIfaINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceAssetIfa) Lower(value []AssetIfa) C.RustBuffer {
	return LowerIntoRustBuffer[[]AssetIfa](c, value)
}

func (c FfiConverterSequenceAssetIfa) Write(writer io.Writer, value []AssetIfa) {
	if len(value) > math.MaxInt32 {
		panic("[]AssetIfa is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterAssetIfaINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceAssetIfa struct{}

func (FfiDestroyerSequenceAssetIfa) Destroy(sequence []AssetIfa) {
	for _, value := range sequence {
		FfiDestroyerAssetIfa{}.Destroy(value)
	}
}

type FfiConverterSequenceAssetNia struct{}

var FfiConverterSequenceAssetNiaINSTANCE = FfiConverterSequenceAssetNia{}

func (c FfiConverterSequenceAssetNia) Lift(rb RustBufferI) []AssetNia {
	return LiftFromRustBuffer[[]AssetNia](c, rb)
}

func (c FfiConverterSequenceAssetNia) Read(reader io.Reader) []AssetNia {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]AssetNia, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterAssetNiaINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceAssetNia) Lower(value []AssetNia) C.RustBuffer {
	return LowerIntoRustBuffer[[]AssetNia](c, value)
}

func (c FfiConverterSequenceAssetNia) Write(writer io.Writer, value []AssetNia) {
	if len(value) > math.MaxInt32 {
		panic("[]AssetNia is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterAssetNiaINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceAssetNia struct{}

func (FfiDestroyerSequenceAssetNia) Destroy(sequence []AssetNia) {
	for _, value := range sequence {
		FfiDestroyerAssetNia{}.Destroy(value)
	}
}

type FfiConverterSequenceAssetUda struct{}

var FfiConverterSequenceAssetUdaINSTANCE = FfiConverterSequenceAssetUda{}

func (c FfiConverterSequenceAssetUda) Lift(rb RustBufferI) []AssetUda {
	return LiftFromRustBuffer[[]AssetUda](c, rb)
}

func (c FfiConverterSequenceAssetUda) Read(reader io.Reader) []AssetUda {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]AssetUda, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterAssetUdaINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceAssetUda) Lower(value []AssetUda) C.RustBuffer {
	return LowerIntoRustBuffer[[]AssetUda](c, value)
}

func (c FfiConverterSequenceAssetUda) Write(writer io.Writer, value []AssetUda) {
	if len(value) > math.MaxInt32 {
		panic("[]AssetUda is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterAssetUdaINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceAssetUda struct{}

func (FfiDestroyerSequenceAssetUda) Destroy(sequence []AssetUda) {
	for _, value := range sequence {
		FfiDestroyerAssetUda{}.Destroy(value)
	}
}

type FfiConverterSequenceRecipient struct{}

var FfiConverterSequenceRecipientINSTANCE = FfiConverterSequenceRecipient{}

func (c FfiConverterSequenceRecipient) Lift(rb RustBufferI) []Recipient {
	return LiftFromRustBuffer[[]Recipient](c, rb)
}

func (c FfiConverterSequenceRecipient) Read(reader io.Reader) []Recipient {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]Recipient, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterRecipientINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceRecipient) Lower(value []Recipient) C.RustBuffer {
	return LowerIntoRustBuffer[[]Recipient](c, value)
}

func (c FfiConverterSequenceRecipient) Write(writer io.Writer, value []Recipient) {
	if len(value) > math.MaxInt32 {
		panic("[]Recipient is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterRecipientINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceRecipient struct{}

func (FfiDestroyerSequenceRecipient) Destroy(sequence []Recipient) {
	for _, value := range sequence {
		FfiDestroyerRecipient{}.Destroy(value)
	}
}

type FfiConverterSequenceRefreshFilter struct{}

var FfiConverterSequenceRefreshFilterINSTANCE = FfiConverterSequenceRefreshFilter{}

func (c FfiConverterSequenceRefreshFilter) Lift(rb RustBufferI) []RefreshFilter {
	return LiftFromRustBuffer[[]RefreshFilter](c, rb)
}

func (c FfiConverterSequenceRefreshFilter) Read(reader io.Reader) []RefreshFilter {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]RefreshFilter, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterRefreshFilterINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceRefreshFilter) Lower(value []RefreshFilter) C.RustBuffer {
	return LowerIntoRustBuffer[[]RefreshFilter](c, value)
}

func (c FfiConverterSequenceRefreshFilter) Write(writer io.Writer, value []RefreshFilter) {
	if len(value) > math.MaxInt32 {
		panic("[]RefreshFilter is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterRefreshFilterINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceRefreshFilter struct{}

func (FfiDestroyerSequenceRefreshFilter) Destroy(sequence []RefreshFilter) {
	for _, value := range sequence {
		FfiDestroyerRefreshFilter{}.Destroy(value)
	}
}

type FfiConverterSequenceRgbAllocation struct{}

var FfiConverterSequenceRgbAllocationINSTANCE = FfiConverterSequenceRgbAllocation{}

func (c FfiConverterSequenceRgbAllocation) Lift(rb RustBufferI) []RgbAllocation {
	return LiftFromRustBuffer[[]RgbAllocation](c, rb)
}

func (c FfiConverterSequenceRgbAllocation) Read(reader io.Reader) []RgbAllocation {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]RgbAllocation, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterRgbAllocationINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceRgbAllocation) Lower(value []RgbAllocation) C.RustBuffer {
	return LowerIntoRustBuffer[[]RgbAllocation](c, value)
}

func (c FfiConverterSequenceRgbAllocation) Write(writer io.Writer, value []RgbAllocation) {
	if len(value) > math.MaxInt32 {
		panic("[]RgbAllocation is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterRgbAllocationINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceRgbAllocation struct{}

func (FfiDestroyerSequenceRgbAllocation) Destroy(sequence []RgbAllocation) {
	for _, value := range sequence {
		FfiDestroyerRgbAllocation{}.Destroy(value)
	}
}

type FfiConverterSequenceTransaction struct{}

var FfiConverterSequenceTransactionINSTANCE = FfiConverterSequenceTransaction{}

func (c FfiConverterSequenceTransaction) Lift(rb RustBufferI) []Transaction {
	return LiftFromRustBuffer[[]Transaction](c, rb)
}

func (c FfiConverterSequenceTransaction) Read(reader io.Reader) []Transaction {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]Transaction, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterTransactionINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceTransaction) Lower(value []Transaction) C.RustBuffer {
	return LowerIntoRustBuffer[[]Transaction](c, value)
}

func (c FfiConverterSequenceTransaction) Write(writer io.Writer, value []Transaction) {
	if len(value) > math.MaxInt32 {
		panic("[]Transaction is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterTransactionINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceTransaction struct{}

func (FfiDestroyerSequenceTransaction) Destroy(sequence []Transaction) {
	for _, value := range sequence {
		FfiDestroyerTransaction{}.Destroy(value)
	}
}

type FfiConverterSequenceTransfer struct{}

var FfiConverterSequenceTransferINSTANCE = FfiConverterSequenceTransfer{}

func (c FfiConverterSequenceTransfer) Lift(rb RustBufferI) []Transfer {
	return LiftFromRustBuffer[[]Transfer](c, rb)
}

func (c FfiConverterSequenceTransfer) Read(reader io.Reader) []Transfer {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]Transfer, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterTransferINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceTransfer) Lower(value []Transfer) C.RustBuffer {
	return LowerIntoRustBuffer[[]Transfer](c, value)
}

func (c FfiConverterSequenceTransfer) Write(writer io.Writer, value []Transfer) {
	if len(value) > math.MaxInt32 {
		panic("[]Transfer is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterTransferINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceTransfer struct{}

func (FfiDestroyerSequenceTransfer) Destroy(sequence []Transfer) {
	for _, value := range sequence {
		FfiDestroyerTransfer{}.Destroy(value)
	}
}

type FfiConverterSequenceTransferTransportEndpoint struct{}

var FfiConverterSequenceTransferTransportEndpointINSTANCE = FfiConverterSequenceTransferTransportEndpoint{}

func (c FfiConverterSequenceTransferTransportEndpoint) Lift(rb RustBufferI) []TransferTransportEndpoint {
	return LiftFromRustBuffer[[]TransferTransportEndpoint](c, rb)
}

func (c FfiConverterSequenceTransferTransportEndpoint) Read(reader io.Reader) []TransferTransportEndpoint {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]TransferTransportEndpoint, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterTransferTransportEndpointINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceTransferTransportEndpoint) Lower(value []TransferTransportEndpoint) C.RustBuffer {
	return LowerIntoRustBuffer[[]TransferTransportEndpoint](c, value)
}

func (c FfiConverterSequenceTransferTransportEndpoint) Write(writer io.Writer, value []TransferTransportEndpoint) {
	if len(value) > math.MaxInt32 {
		panic("[]TransferTransportEndpoint is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterTransferTransportEndpointINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceTransferTransportEndpoint struct{}

func (FfiDestroyerSequenceTransferTransportEndpoint) Destroy(sequence []TransferTransportEndpoint) {
	for _, value := range sequence {
		FfiDestroyerTransferTransportEndpoint{}.Destroy(value)
	}
}

type FfiConverterSequenceUnspent struct{}

var FfiConverterSequenceUnspentINSTANCE = FfiConverterSequenceUnspent{}

func (c FfiConverterSequenceUnspent) Lift(rb RustBufferI) []Unspent {
	return LiftFromRustBuffer[[]Unspent](c, rb)
}

func (c FfiConverterSequenceUnspent) Read(reader io.Reader) []Unspent {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]Unspent, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterUnspentINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceUnspent) Lower(value []Unspent) C.RustBuffer {
	return LowerIntoRustBuffer[[]Unspent](c, value)
}

func (c FfiConverterSequenceUnspent) Write(writer io.Writer, value []Unspent) {
	if len(value) > math.MaxInt32 {
		panic("[]Unspent is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterUnspentINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceUnspent struct{}

func (FfiDestroyerSequenceUnspent) Destroy(sequence []Unspent) {
	for _, value := range sequence {
		FfiDestroyerUnspent{}.Destroy(value)
	}
}

type FfiConverterSequenceAssetSchema struct{}

var FfiConverterSequenceAssetSchemaINSTANCE = FfiConverterSequenceAssetSchema{}

func (c FfiConverterSequenceAssetSchema) Lift(rb RustBufferI) []AssetSchema {
	return LiftFromRustBuffer[[]AssetSchema](c, rb)
}

func (c FfiConverterSequenceAssetSchema) Read(reader io.Reader) []AssetSchema {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]AssetSchema, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterAssetSchemaINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceAssetSchema) Lower(value []AssetSchema) C.RustBuffer {
	return LowerIntoRustBuffer[[]AssetSchema](c, value)
}

func (c FfiConverterSequenceAssetSchema) Write(writer io.Writer, value []AssetSchema) {
	if len(value) > math.MaxInt32 {
		panic("[]AssetSchema is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterAssetSchemaINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceAssetSchema struct{}

func (FfiDestroyerSequenceAssetSchema) Destroy(sequence []AssetSchema) {
	for _, value := range sequence {
		FfiDestroyerAssetSchema{}.Destroy(value)
	}
}

type FfiConverterSequenceAssignment struct{}

var FfiConverterSequenceAssignmentINSTANCE = FfiConverterSequenceAssignment{}

func (c FfiConverterSequenceAssignment) Lift(rb RustBufferI) []Assignment {
	return LiftFromRustBuffer[[]Assignment](c, rb)
}

func (c FfiConverterSequenceAssignment) Read(reader io.Reader) []Assignment {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]Assignment, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterAssignmentINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceAssignment) Lower(value []Assignment) C.RustBuffer {
	return LowerIntoRustBuffer[[]Assignment](c, value)
}

func (c FfiConverterSequenceAssignment) Write(writer io.Writer, value []Assignment) {
	if len(value) > math.MaxInt32 {
		panic("[]Assignment is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterAssignmentINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceAssignment struct{}

func (FfiDestroyerSequenceAssignment) Destroy(sequence []Assignment) {
	for _, value := range sequence {
		FfiDestroyerAssignment{}.Destroy(value)
	}
}

type FfiConverterMapUint8Media struct{}

var FfiConverterMapUint8MediaINSTANCE = FfiConverterMapUint8Media{}

func (c FfiConverterMapUint8Media) Lift(rb RustBufferI) map[uint8]Media {
	return LiftFromRustBuffer[map[uint8]Media](c, rb)
}

func (_ FfiConverterMapUint8Media) Read(reader io.Reader) map[uint8]Media {
	result := make(map[uint8]Media)
	length := readInt32(reader)
	for i := int32(0); i < length; i++ {
		key := FfiConverterUint8INSTANCE.Read(reader)
		value := FfiConverterMediaINSTANCE.Read(reader)
		result[key] = value
	}
	return result
}

func (c FfiConverterMapUint8Media) Lower(value map[uint8]Media) C.RustBuffer {
	return LowerIntoRustBuffer[map[uint8]Media](c, value)
}

func (_ FfiConverterMapUint8Media) Write(writer io.Writer, mapValue map[uint8]Media) {
	if len(mapValue) > math.MaxInt32 {
		panic("map[uint8]Media is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(mapValue)))
	for key, value := range mapValue {
		FfiConverterUint8INSTANCE.Write(writer, key)
		FfiConverterMediaINSTANCE.Write(writer, value)
	}
}

type FfiDestroyerMapUint8Media struct{}

func (_ FfiDestroyerMapUint8Media) Destroy(mapValue map[uint8]Media) {
	for key, value := range mapValue {
		FfiDestroyerUint8{}.Destroy(key)
		FfiDestroyerMedia{}.Destroy(value)
	}
}

type FfiConverterMapInt32RefreshedTransfer struct{}

var FfiConverterMapInt32RefreshedTransferINSTANCE = FfiConverterMapInt32RefreshedTransfer{}

func (c FfiConverterMapInt32RefreshedTransfer) Lift(rb RustBufferI) map[int32]RefreshedTransfer {
	return LiftFromRustBuffer[map[int32]RefreshedTransfer](c, rb)
}

func (_ FfiConverterMapInt32RefreshedTransfer) Read(reader io.Reader) map[int32]RefreshedTransfer {
	result := make(map[int32]RefreshedTransfer)
	length := readInt32(reader)
	for i := int32(0); i < length; i++ {
		key := FfiConverterInt32INSTANCE.Read(reader)
		value := FfiConverterRefreshedTransferINSTANCE.Read(reader)
		result[key] = value
	}
	return result
}

func (c FfiConverterMapInt32RefreshedTransfer) Lower(value map[int32]RefreshedTransfer) C.RustBuffer {
	return LowerIntoRustBuffer[map[int32]RefreshedTransfer](c, value)
}

func (_ FfiConverterMapInt32RefreshedTransfer) Write(writer io.Writer, mapValue map[int32]RefreshedTransfer) {
	if len(mapValue) > math.MaxInt32 {
		panic("map[int32]RefreshedTransfer is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(mapValue)))
	for key, value := range mapValue {
		FfiConverterInt32INSTANCE.Write(writer, key)
		FfiConverterRefreshedTransferINSTANCE.Write(writer, value)
	}
}

type FfiDestroyerMapInt32RefreshedTransfer struct{}

func (_ FfiDestroyerMapInt32RefreshedTransfer) Destroy(mapValue map[int32]RefreshedTransfer) {
	for key, value := range mapValue {
		FfiDestroyerInt32{}.Destroy(key)
		FfiDestroyerRefreshedTransfer{}.Destroy(value)
	}
}

type FfiConverterMapStringSequenceRecipient struct{}

var FfiConverterMapStringSequenceRecipientINSTANCE = FfiConverterMapStringSequenceRecipient{}

func (c FfiConverterMapStringSequenceRecipient) Lift(rb RustBufferI) map[string][]Recipient {
	return LiftFromRustBuffer[map[string][]Recipient](c, rb)
}

func (_ FfiConverterMapStringSequenceRecipient) Read(reader io.Reader) map[string][]Recipient {
	result := make(map[string][]Recipient)
	length := readInt32(reader)
	for i := int32(0); i < length; i++ {
		key := FfiConverterStringINSTANCE.Read(reader)
		value := FfiConverterSequenceRecipientINSTANCE.Read(reader)
		result[key] = value
	}
	return result
}

func (c FfiConverterMapStringSequenceRecipient) Lower(value map[string][]Recipient) C.RustBuffer {
	return LowerIntoRustBuffer[map[string][]Recipient](c, value)
}

func (_ FfiConverterMapStringSequenceRecipient) Write(writer io.Writer, mapValue map[string][]Recipient) {
	if len(mapValue) > math.MaxInt32 {
		panic("map[string][]Recipient is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(mapValue)))
	for key, value := range mapValue {
		FfiConverterStringINSTANCE.Write(writer, key)
		FfiConverterSequenceRecipientINSTANCE.Write(writer, value)
	}
}

type FfiDestroyerMapStringSequenceRecipient struct{}

func (_ FfiDestroyerMapStringSequenceRecipient) Destroy(mapValue map[string][]Recipient) {
	for key, value := range mapValue {
		FfiDestroyerString{}.Destroy(key)
		FfiDestroyerSequenceRecipient{}.Destroy(value)
	}
}

func GenerateKeys(bitcoinNetwork BitcoinNetwork) Keys {
	return FfiConverterKeysINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_func_generate_keys(FfiConverterBitcoinNetworkINSTANCE.Lower(bitcoinNetwork), _uniffiStatus),
		}
	}))
}

func RestoreBackup(backupPath string, password string, dataDir string) error {
	_, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_rgblibuniffi_fn_func_restore_backup(FfiConverterStringINSTANCE.Lower(backupPath), FfiConverterStringINSTANCE.Lower(password), FfiConverterStringINSTANCE.Lower(dataDir), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

func RestoreKeys(bitcoinNetwork BitcoinNetwork, mnemonic string) (Keys, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[RgbLibError](FfiConverterRgbLibError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_rgblibuniffi_fn_func_restore_keys(FfiConverterBitcoinNetworkINSTANCE.Lower(bitcoinNetwork), FfiConverterStringINSTANCE.Lower(mnemonic), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue Keys
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterKeysINSTANCE.Lift(_uniffiRV), nil
	}
}
