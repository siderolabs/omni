// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        v5.29.4
// source: common/omni.proto

package common

import (
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"

	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// Data source.
type Runtime int32

const (
	// Kubernetes control plane.
	Runtime_Kubernetes Runtime = 0
	// Talos apid.
	Runtime_Talos Runtime = 1
	// Omni internal runtime.
	Runtime_Omni Runtime = 2
)

// Enum value maps for Runtime.
var (
	Runtime_name = map[int32]string{
		0: "Kubernetes",
		1: "Talos",
		2: "Omni",
	}
	Runtime_value = map[string]int32{
		"Kubernetes": 0,
		"Talos":      1,
		"Omni":       2,
	}
)

func (x Runtime) Enum() *Runtime {
	p := new(Runtime)
	*p = x
	return p
}

func (x Runtime) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Runtime) Descriptor() protoreflect.EnumDescriptor {
	return file_common_omni_proto_enumTypes[0].Descriptor()
}

func (Runtime) Type() protoreflect.EnumType {
	return &file_common_omni_proto_enumTypes[0]
}

func (x Runtime) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Runtime.Descriptor instead.
func (Runtime) EnumDescriptor() ([]byte, []int) {
	return file_common_omni_proto_rawDescGZIP(), []int{0}
}

// Context represents Kubernetes or Talos config source.
type Context struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Name fetches the config from the top level Kubeconfig or Talosconfig.
	Name          string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Context) Reset() {
	*x = Context{}
	mi := &file_common_omni_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Context) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Context) ProtoMessage() {}

func (x *Context) ProtoReflect() protoreflect.Message {
	mi := &file_common_omni_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Context.ProtoReflect.Descriptor instead.
func (*Context) Descriptor() ([]byte, []int) {
	return file_common_omni_proto_rawDescGZIP(), []int{0}
}

func (x *Context) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

var File_common_omni_proto protoreflect.FileDescriptor

const file_common_omni_proto_rawDesc = "" +
	"\n" +
	"\x11common/omni.proto\x12\x06common\")\n" +
	"\aContext\x12\x12\n" +
	"\x04name\x18\x01 \x01(\tR\x04nameJ\x04\b\x02\x10\x03J\x04\b\x03\x10\x04*.\n" +
	"\aRuntime\x12\x0e\n" +
	"\n" +
	"Kubernetes\x10\x00\x12\t\n" +
	"\x05Talos\x10\x01\x12\b\n" +
	"\x04Omni\x10\x02B.Z,github.com/siderolabs/omni/client/api/commonb\x06proto3"

var (
	file_common_omni_proto_rawDescOnce sync.Once
	file_common_omni_proto_rawDescData []byte
)

func file_common_omni_proto_rawDescGZIP() []byte {
	file_common_omni_proto_rawDescOnce.Do(func() {
		file_common_omni_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_common_omni_proto_rawDesc), len(file_common_omni_proto_rawDesc)))
	})
	return file_common_omni_proto_rawDescData
}

var file_common_omni_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_common_omni_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_common_omni_proto_goTypes = []any{
	(Runtime)(0),    // 0: common.Runtime
	(*Context)(nil), // 1: common.Context
}
var file_common_omni_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_common_omni_proto_init() }
func file_common_omni_proto_init() {
	if File_common_omni_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_common_omni_proto_rawDesc), len(file_common_omni_proto_rawDesc)),
			NumEnums:      1,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_common_omni_proto_goTypes,
		DependencyIndexes: file_common_omni_proto_depIdxs,
		EnumInfos:         file_common_omni_proto_enumTypes,
		MessageInfos:      file_common_omni_proto_msgTypes,
	}.Build()
	File_common_omni_proto = out.File
	file_common_omni_proto_goTypes = nil
	file_common_omni_proto_depIdxs = nil
}
