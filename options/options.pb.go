// Code generated by protoc-gen-go. DO NOT EDIT.
// source: options.proto

package options

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	descriptor "github.com/golang/protobuf/protoc-gen-go/descriptor"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type Schema struct {
	Federated            bool     `protobuf:"varint,1,opt,name=federated,proto3" json:"federated,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Schema) Reset()         { *m = Schema{} }
func (m *Schema) String() string { return proto.CompactTextString(m) }
func (*Schema) ProtoMessage()    {}
func (*Schema) Descriptor() ([]byte, []int) {
	return fileDescriptor_110d40819f1994f9, []int{0}
}

func (m *Schema) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Schema.Unmarshal(m, b)
}
func (m *Schema) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Schema.Marshal(b, m, deterministic)
}
func (m *Schema) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Schema.Merge(m, src)
}
func (m *Schema) XXX_Size() int {
	return xxx_messageInfo_Schema.Size(m)
}
func (m *Schema) XXX_DiscardUnknown() {
	xxx_messageInfo_Schema.DiscardUnknown(m)
}

var xxx_messageInfo_Schema proto.InternalMessageInfo

func (m *Schema) GetFederated() bool {
	if m != nil {
		return m.Federated
	}
	return false
}

type RPC struct {
	Mutation             bool     `protobuf:"varint,1,opt,name=mutation,proto3" json:"mutation,omitempty"`
	Skip                 bool     `protobuf:"varint,2,opt,name=skip,proto3" json:"skip,omitempty"`
	RespondsWith         []string `protobuf:"bytes,3,rep,name=respondsWith,proto3" json:"respondsWith,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *RPC) Reset()         { *m = RPC{} }
func (m *RPC) String() string { return proto.CompactTextString(m) }
func (*RPC) ProtoMessage()    {}
func (*RPC) Descriptor() ([]byte, []int) {
	return fileDescriptor_110d40819f1994f9, []int{1}
}

func (m *RPC) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RPC.Unmarshal(m, b)
}
func (m *RPC) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RPC.Marshal(b, m, deterministic)
}
func (m *RPC) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RPC.Merge(m, src)
}
func (m *RPC) XXX_Size() int {
	return xxx_messageInfo_RPC.Size(m)
}
func (m *RPC) XXX_DiscardUnknown() {
	xxx_messageInfo_RPC.DiscardUnknown(m)
}

var xxx_messageInfo_RPC proto.InternalMessageInfo

func (m *RPC) GetMutation() bool {
	if m != nil {
		return m.Mutation
	}
	return false
}

func (m *RPC) GetSkip() bool {
	if m != nil {
		return m.Skip
	}
	return false
}

func (m *RPC) GetRespondsWith() []string {
	if m != nil {
		return m.RespondsWith
	}
	return nil
}

var E_Rpc = &proto.ExtensionDesc{
	ExtendedType:  (*descriptor.MethodOptions)(nil),
	ExtensionType: (*RPC)(nil),
	Field:         1070,
	Name:          "twirpql.options.rpc",
	Tag:           "bytes,1070,opt,name=rpc",
	Filename:      "options.proto",
}

var E_Schema = &proto.ExtensionDesc{
	ExtendedType:  (*descriptor.FileOptions)(nil),
	ExtensionType: (*Schema)(nil),
	Field:         1070,
	Name:          "twirpql.options.schema",
	Tag:           "bytes,1070,opt,name=schema",
	Filename:      "options.proto",
}

func init() {
	proto.RegisterType((*Schema)(nil), "twirpql.options.Schema")
	proto.RegisterType((*RPC)(nil), "twirpql.options.RPC")
	proto.RegisterExtension(E_Rpc)
	proto.RegisterExtension(E_Schema)
}

func init() { proto.RegisterFile("options.proto", fileDescriptor_110d40819f1994f9) }

var fileDescriptor_110d40819f1994f9 = []byte{
	// 268 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x64, 0x90, 0xc1, 0x4b, 0xc3, 0x30,
	0x14, 0xc6, 0xa9, 0x95, 0xd2, 0x45, 0x45, 0x08, 0x82, 0x65, 0x88, 0x94, 0x1e, 0xa4, 0x07, 0x97,
	0x82, 0xde, 0xe6, 0xcd, 0x81, 0x9e, 0xd4, 0x11, 0x0f, 0xa2, 0xb7, 0xae, 0x79, 0x6b, 0x83, 0x6d,
	0x5f, 0x4c, 0x32, 0xf6, 0x5f, 0xf9, 0x37, 0xca, 0x9a, 0xa8, 0x6c, 0x3b, 0x25, 0xf9, 0xf2, 0xbe,
	0x1f, 0xef, 0xfb, 0xc8, 0x09, 0x2a, 0x2b, 0xb1, 0x37, 0x4c, 0x69, 0xb4, 0x48, 0x4f, 0xed, 0x5a,
	0x6a, 0xf5, 0xd5, 0x32, 0x2f, 0x8f, 0xd3, 0x1a, 0xb1, 0x6e, 0xa1, 0x18, 0xbe, 0x17, 0xab, 0x65,
	0x21, 0xc0, 0x54, 0x5a, 0x2a, 0x8b, 0xda, 0x59, 0xb2, 0x2b, 0x12, 0xbd, 0x56, 0x0d, 0x74, 0x25,
	0xbd, 0x20, 0xa3, 0x25, 0x08, 0xd0, 0xa5, 0x05, 0x91, 0x04, 0x69, 0x90, 0xc7, 0xfc, 0x5f, 0xc8,
	0xde, 0x49, 0xc8, 0xe7, 0x33, 0x3a, 0x26, 0x71, 0xb7, 0xb2, 0xe5, 0x86, 0xee, 0x67, 0xfe, 0xde,
	0x94, 0x92, 0x43, 0xf3, 0x29, 0x55, 0x72, 0x30, 0xe8, 0xc3, 0x9d, 0x66, 0xe4, 0x58, 0x83, 0x51,
	0xd8, 0x0b, 0xf3, 0x26, 0x6d, 0x93, 0x84, 0x69, 0x98, 0x8f, 0xf8, 0x96, 0x36, 0x7d, 0x24, 0xa1,
	0x56, 0x15, 0xbd, 0x64, 0x6e, 0x59, 0xf6, 0xbb, 0x2c, 0x7b, 0x02, 0xdb, 0xa0, 0x78, 0x71, 0x59,
	0x92, 0xef, 0x38, 0x0d, 0xf2, 0xa3, 0x9b, 0x33, 0xb6, 0x13, 0x92, 0xf1, 0xf9, 0x8c, 0x6f, 0x08,
	0xd3, 0x67, 0x12, 0x19, 0x9f, 0x65, 0x8f, 0xf5, 0x20, 0x5b, 0xd8, 0x21, 0x9d, 0xef, 0x91, 0x5c,
	0x13, 0xdc, 0x53, 0xee, 0xd9, 0xc7, 0x75, 0x57, 0xea, 0x75, 0xd9, 0x33, 0x89, 0xae, 0xc2, 0x6a,
	0x52, 0x43, 0x3f, 0xf1, 0xb6, 0xc2, 0xdb, 0xee, 0xfc, 0xb9, 0x88, 0x86, 0x99, 0xdb, 0x9f, 0x00,
	0x00, 0x00, 0xff, 0xff, 0x16, 0x1b, 0x30, 0xbb, 0x96, 0x01, 0x00, 0x00,
}
