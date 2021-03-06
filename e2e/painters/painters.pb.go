// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.25.0
// 	protoc        v3.12.4
// source: painters.proto

package painters

import (
	proto "github.com/golang/protobuf/proto"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// This is a compile-time assertion that a sufficiently up-to-date version
// of the legacy proto package is being used.
const _ = proto.ProtoPackageIsVersion4

type Painter struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"` // TODO: add painter's style as an enum
}

func (x *Painter) Reset() {
	*x = Painter{}
	if protoimpl.UnsafeEnabled {
		mi := &file_painters_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Painter) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Painter) ProtoMessage() {}

func (x *Painter) ProtoReflect() protoreflect.Message {
	mi := &file_painters_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Painter.ProtoReflect.Descriptor instead.
func (*Painter) Descriptor() ([]byte, []int) {
	return file_painters_proto_rawDescGZIP(), []int{0}
}

func (x *Painter) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

var File_painters_proto protoreflect.FileDescriptor

var file_painters_proto_rawDesc = []byte{
	0x0a, 0x0e, 0x70, 0x61, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x12, 0x08, 0x70, 0x61, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x73, 0x22, 0x1d, 0x0a, 0x07, 0x50, 0x61,
	0x69, 0x6e, 0x74, 0x65, 0x72, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x42, 0x30, 0x5a, 0x2e, 0x67, 0x69, 0x74,
	0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x74, 0x6d, 0x63, 0x2f, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x63, 0x2d, 0x67, 0x65, 0x6e, 0x2d, 0x67, 0x72, 0x61, 0x70, 0x68, 0x71, 0x6c, 0x2f, 0x65,
	0x32, 0x65, 0x2f, 0x70, 0x61, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x73, 0x62, 0x06, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x33,
}

var (
	file_painters_proto_rawDescOnce sync.Once
	file_painters_proto_rawDescData = file_painters_proto_rawDesc
)

func file_painters_proto_rawDescGZIP() []byte {
	file_painters_proto_rawDescOnce.Do(func() {
		file_painters_proto_rawDescData = protoimpl.X.CompressGZIP(file_painters_proto_rawDescData)
	})
	return file_painters_proto_rawDescData
}

var file_painters_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_painters_proto_goTypes = []interface{}{
	(*Painter)(nil), // 0: painters.Painter
}
var file_painters_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_painters_proto_init() }
func file_painters_proto_init() {
	if File_painters_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_painters_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Painter); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_painters_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_painters_proto_goTypes,
		DependencyIndexes: file_painters_proto_depIdxs,
		MessageInfos:      file_painters_proto_msgTypes,
	}.Build()
	File_painters_proto = out.File
	file_painters_proto_rawDesc = nil
	file_painters_proto_goTypes = nil
	file_painters_proto_depIdxs = nil
}
