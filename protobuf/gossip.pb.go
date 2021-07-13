// Code generated by protoc-gen-go. DO NOT EDIT.
// source: gossip.proto

package proto

import (
	context "context"
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
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

type State struct {
	//repeated NodeInfo existingNodes
	ExistingHosts        map[string]uint64 `protobuf:"bytes,1,rep,name=existingHosts,proto3" json:"existingHosts,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"varint,2,opt,name=value,proto3"`
	OwnNote              *Note             `protobuf:"bytes,2,opt,name=ownNote,proto3" json:"ownNote,omitempty"`
	ExternalGossip       []byte            `protobuf:"bytes,3,opt,name=externalGossip,proto3" json:"externalGossip,omitempty"`
	XXX_NoUnkeyedLiteral struct{}          `json:"-"`
	XXX_unrecognized     []byte            `json:"-"`
	XXX_sizecache        int32             `json:"-"`
}

func (m *State) Reset()         { *m = State{} }
func (m *State) String() string { return proto.CompactTextString(m) }
func (*State) ProtoMessage()    {}
func (*State) Descriptor() ([]byte, []int) {
	return fileDescriptor_878fa4887b90140c, []int{0}
}

func (m *State) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_State.Unmarshal(m, b)
}
func (m *State) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_State.Marshal(b, m, deterministic)
}
func (m *State) XXX_Merge(src proto.Message) {
	xxx_messageInfo_State.Merge(m, src)
}
func (m *State) XXX_Size() int {
	return xxx_messageInfo_State.Size(m)
}
func (m *State) XXX_DiscardUnknown() {
	xxx_messageInfo_State.DiscardUnknown(m)
}

var xxx_messageInfo_State proto.InternalMessageInfo

func (m *State) GetExistingHosts() map[string]uint64 {
	if m != nil {
		return m.ExistingHosts
	}
	return nil
}

func (m *State) GetOwnNote() *Note {
	if m != nil {
		return m.OwnNote
	}
	return nil
}

func (m *State) GetExternalGossip() []byte {
	if m != nil {
		return m.ExternalGossip
	}
	return nil
}

//Application message
type Msg struct {
	Content              []byte   `protobuf:"bytes,1,opt,name=content,proto3" json:"content,omitempty"`
	Error                string   `protobuf:"bytes,2,opt,name=error,proto3" json:"error,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Msg) Reset()         { *m = Msg{} }
func (m *Msg) String() string { return proto.CompactTextString(m) }
func (*Msg) ProtoMessage()    {}
func (*Msg) Descriptor() ([]byte, []int) {
	return fileDescriptor_878fa4887b90140c, []int{1}
}

func (m *Msg) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Msg.Unmarshal(m, b)
}
func (m *Msg) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Msg.Marshal(b, m, deterministic)
}
func (m *Msg) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Msg.Merge(m, src)
}
func (m *Msg) XXX_Size() int {
	return xxx_messageInfo_Msg.Size(m)
}
func (m *Msg) XXX_DiscardUnknown() {
	xxx_messageInfo_Msg.DiscardUnknown(m)
}

var xxx_messageInfo_Msg proto.InternalMessageInfo

func (m *Msg) GetContent() []byte {
	if m != nil {
		return m.Content
	}
	return nil
}

func (m *Msg) GetError() string {
	if m != nil {
		return m.Error
	}
	return ""
}

//Application response
type MsgResponse struct {
	Content              []byte   `protobuf:"bytes,1,opt,name=content,proto3" json:"content,omitempty"`
	Error                string   `protobuf:"bytes,2,opt,name=error,proto3" json:"error,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *MsgResponse) Reset()         { *m = MsgResponse{} }
func (m *MsgResponse) String() string { return proto.CompactTextString(m) }
func (*MsgResponse) ProtoMessage()    {}
func (*MsgResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_878fa4887b90140c, []int{2}
}

func (m *MsgResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_MsgResponse.Unmarshal(m, b)
}
func (m *MsgResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_MsgResponse.Marshal(b, m, deterministic)
}
func (m *MsgResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_MsgResponse.Merge(m, src)
}
func (m *MsgResponse) XXX_Size() int {
	return xxx_messageInfo_MsgResponse.Size(m)
}
func (m *MsgResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_MsgResponse.DiscardUnknown(m)
}

var xxx_messageInfo_MsgResponse proto.InternalMessageInfo

func (m *MsgResponse) GetContent() []byte {
	if m != nil {
		return m.Content
	}
	return nil
}

func (m *MsgResponse) GetError() string {
	if m != nil {
		return m.Error
	}
	return ""
}

type StateResponse struct {
	Certificates         []*Certificate `protobuf:"bytes,1,rep,name=certificates,proto3" json:"certificates,omitempty"`
	Notes                []*Note        `protobuf:"bytes,2,rep,name=notes,proto3" json:"notes,omitempty"`
	Accusations          []*Accusation  `protobuf:"bytes,3,rep,name=accusations,proto3" json:"accusations,omitempty"`
	ExternalGossip       []byte         `protobuf:"bytes,4,opt,name=externalGossip,proto3" json:"externalGossip,omitempty"`
	XXX_NoUnkeyedLiteral struct{}       `json:"-"`
	XXX_unrecognized     []byte         `json:"-"`
	XXX_sizecache        int32          `json:"-"`
}

func (m *StateResponse) Reset()         { *m = StateResponse{} }
func (m *StateResponse) String() string { return proto.CompactTextString(m) }
func (*StateResponse) ProtoMessage()    {}
func (*StateResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_878fa4887b90140c, []int{3}
}

func (m *StateResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_StateResponse.Unmarshal(m, b)
}
func (m *StateResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_StateResponse.Marshal(b, m, deterministic)
}
func (m *StateResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_StateResponse.Merge(m, src)
}
func (m *StateResponse) XXX_Size() int {
	return xxx_messageInfo_StateResponse.Size(m)
}
func (m *StateResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_StateResponse.DiscardUnknown(m)
}

var xxx_messageInfo_StateResponse proto.InternalMessageInfo

func (m *StateResponse) GetCertificates() []*Certificate {
	if m != nil {
		return m.Certificates
	}
	return nil
}

func (m *StateResponse) GetNotes() []*Note {
	if m != nil {
		return m.Notes
	}
	return nil
}

func (m *StateResponse) GetAccusations() []*Accusation {
	if m != nil {
		return m.Accusations
	}
	return nil
}

func (m *StateResponse) GetExternalGossip() []byte {
	if m != nil {
		return m.ExternalGossip
	}
	return nil
}

//Raw certificate
type Certificate struct {
	Raw                  []byte   `protobuf:"bytes,1,opt,name=raw,proto3" json:"raw,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Certificate) Reset()         { *m = Certificate{} }
func (m *Certificate) String() string { return proto.CompactTextString(m) }
func (*Certificate) ProtoMessage()    {}
func (*Certificate) Descriptor() ([]byte, []int) {
	return fileDescriptor_878fa4887b90140c, []int{4}
}

func (m *Certificate) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Certificate.Unmarshal(m, b)
}
func (m *Certificate) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Certificate.Marshal(b, m, deterministic)
}
func (m *Certificate) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Certificate.Merge(m, src)
}
func (m *Certificate) XXX_Size() int {
	return xxx_messageInfo_Certificate.Size(m)
}
func (m *Certificate) XXX_DiscardUnknown() {
	xxx_messageInfo_Certificate.DiscardUnknown(m)
}

var xxx_messageInfo_Certificate proto.InternalMessageInfo

func (m *Certificate) GetRaw() []byte {
	if m != nil {
		return m.Raw
	}
	return nil
}

//accuser and accused are the respective node ids
type Accusation struct {
	Epoch                uint64     `protobuf:"varint,1,opt,name=epoch,proto3" json:"epoch,omitempty"`
	Accuser              []byte     `protobuf:"bytes,2,opt,name=accuser,proto3" json:"accuser,omitempty"`
	Accused              []byte     `protobuf:"bytes,3,opt,name=accused,proto3" json:"accused,omitempty"`
	Signature            *Signature `protobuf:"bytes,4,opt,name=signature,proto3" json:"signature,omitempty"`
	RingNum              uint32     `protobuf:"varint,5,opt,name=ringNum,proto3" json:"ringNum,omitempty"`
	XXX_NoUnkeyedLiteral struct{}   `json:"-"`
	XXX_unrecognized     []byte     `json:"-"`
	XXX_sizecache        int32      `json:"-"`
}

func (m *Accusation) Reset()         { *m = Accusation{} }
func (m *Accusation) String() string { return proto.CompactTextString(m) }
func (*Accusation) ProtoMessage()    {}
func (*Accusation) Descriptor() ([]byte, []int) {
	return fileDescriptor_878fa4887b90140c, []int{5}
}

func (m *Accusation) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Accusation.Unmarshal(m, b)
}
func (m *Accusation) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Accusation.Marshal(b, m, deterministic)
}
func (m *Accusation) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Accusation.Merge(m, src)
}
func (m *Accusation) XXX_Size() int {
	return xxx_messageInfo_Accusation.Size(m)
}
func (m *Accusation) XXX_DiscardUnknown() {
	xxx_messageInfo_Accusation.DiscardUnknown(m)
}

var xxx_messageInfo_Accusation proto.InternalMessageInfo

func (m *Accusation) GetEpoch() uint64 {
	if m != nil {
		return m.Epoch
	}
	return 0
}

func (m *Accusation) GetAccuser() []byte {
	if m != nil {
		return m.Accuser
	}
	return nil
}

func (m *Accusation) GetAccused() []byte {
	if m != nil {
		return m.Accused
	}
	return nil
}

func (m *Accusation) GetSignature() *Signature {
	if m != nil {
		return m.Signature
	}
	return nil
}

func (m *Accusation) GetRingNum() uint32 {
	if m != nil {
		return m.RingNum
	}
	return 0
}

type Note struct {
	Epoch                uint64     `protobuf:"varint,1,opt,name=epoch,proto3" json:"epoch,omitempty"`
	Id                   []byte     `protobuf:"bytes,2,opt,name=id,proto3" json:"id,omitempty"`
	Mask                 uint32     `protobuf:"varint,3,opt,name=mask,proto3" json:"mask,omitempty"`
	Signature            *Signature `protobuf:"bytes,4,opt,name=signature,proto3" json:"signature,omitempty"`
	XXX_NoUnkeyedLiteral struct{}   `json:"-"`
	XXX_unrecognized     []byte     `json:"-"`
	XXX_sizecache        int32      `json:"-"`
}

func (m *Note) Reset()         { *m = Note{} }
func (m *Note) String() string { return proto.CompactTextString(m) }
func (*Note) ProtoMessage()    {}
func (*Note) Descriptor() ([]byte, []int) {
	return fileDescriptor_878fa4887b90140c, []int{6}
}

func (m *Note) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Note.Unmarshal(m, b)
}
func (m *Note) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Note.Marshal(b, m, deterministic)
}
func (m *Note) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Note.Merge(m, src)
}
func (m *Note) XXX_Size() int {
	return xxx_messageInfo_Note.Size(m)
}
func (m *Note) XXX_DiscardUnknown() {
	xxx_messageInfo_Note.DiscardUnknown(m)
}

var xxx_messageInfo_Note proto.InternalMessageInfo

func (m *Note) GetEpoch() uint64 {
	if m != nil {
		return m.Epoch
	}
	return 0
}

func (m *Note) GetId() []byte {
	if m != nil {
		return m.Id
	}
	return nil
}

func (m *Note) GetMask() uint32 {
	if m != nil {
		return m.Mask
	}
	return 0
}

func (m *Note) GetSignature() *Signature {
	if m != nil {
		return m.Signature
	}
	return nil
}

//Raw elliptic signature
type Signature struct {
	R                    []byte   `protobuf:"bytes,1,opt,name=r,proto3" json:"r,omitempty"`
	S                    []byte   `protobuf:"bytes,2,opt,name=s,proto3" json:"s,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Signature) Reset()         { *m = Signature{} }
func (m *Signature) String() string { return proto.CompactTextString(m) }
func (*Signature) ProtoMessage()    {}
func (*Signature) Descriptor() ([]byte, []int) {
	return fileDescriptor_878fa4887b90140c, []int{7}
}

func (m *Signature) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Signature.Unmarshal(m, b)
}
func (m *Signature) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Signature.Marshal(b, m, deterministic)
}
func (m *Signature) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Signature.Merge(m, src)
}
func (m *Signature) XXX_Size() int {
	return xxx_messageInfo_Signature.Size(m)
}
func (m *Signature) XXX_DiscardUnknown() {
	xxx_messageInfo_Signature.DiscardUnknown(m)
}

var xxx_messageInfo_Signature proto.InternalMessageInfo

func (m *Signature) GetR() []byte {
	if m != nil {
		return m.R
	}
	return nil
}

func (m *Signature) GetS() []byte {
	if m != nil {
		return m.S
	}
	return nil
}

type Data struct {
	Content              []byte   `protobuf:"bytes,1,opt,name=content,proto3" json:"content,omitempty"`
	Id                   []byte   `protobuf:"bytes,2,opt,name=id,proto3" json:"id,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Data) Reset()         { *m = Data{} }
func (m *Data) String() string { return proto.CompactTextString(m) }
func (*Data) ProtoMessage()    {}
func (*Data) Descriptor() ([]byte, []int) {
	return fileDescriptor_878fa4887b90140c, []int{8}
}

func (m *Data) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Data.Unmarshal(m, b)
}
func (m *Data) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Data.Marshal(b, m, deterministic)
}
func (m *Data) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Data.Merge(m, src)
}
func (m *Data) XXX_Size() int {
	return xxx_messageInfo_Data.Size(m)
}
func (m *Data) XXX_DiscardUnknown() {
	xxx_messageInfo_Data.DiscardUnknown(m)
}

var xxx_messageInfo_Data proto.InternalMessageInfo

func (m *Data) GetContent() []byte {
	if m != nil {
		return m.Content
	}
	return nil
}

func (m *Data) GetId() []byte {
	if m != nil {
		return m.Id
	}
	return nil
}

type Ping struct {
	Nonce                []byte   `protobuf:"bytes,1,opt,name=nonce,proto3" json:"nonce,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Ping) Reset()         { *m = Ping{} }
func (m *Ping) String() string { return proto.CompactTextString(m) }
func (*Ping) ProtoMessage()    {}
func (*Ping) Descriptor() ([]byte, []int) {
	return fileDescriptor_878fa4887b90140c, []int{9}
}

func (m *Ping) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Ping.Unmarshal(m, b)
}
func (m *Ping) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Ping.Marshal(b, m, deterministic)
}
func (m *Ping) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Ping.Merge(m, src)
}
func (m *Ping) XXX_Size() int {
	return xxx_messageInfo_Ping.Size(m)
}
func (m *Ping) XXX_DiscardUnknown() {
	xxx_messageInfo_Ping.DiscardUnknown(m)
}

var xxx_messageInfo_Ping proto.InternalMessageInfo

func (m *Ping) GetNonce() []byte {
	if m != nil {
		return m.Nonce
	}
	return nil
}

type Pong struct {
	Nonce                []byte     `protobuf:"bytes,1,opt,name=nonce,proto3" json:"nonce,omitempty"`
	Signature            *Signature `protobuf:"bytes,2,opt,name=signature,proto3" json:"signature,omitempty"`
	XXX_NoUnkeyedLiteral struct{}   `json:"-"`
	XXX_unrecognized     []byte     `json:"-"`
	XXX_sizecache        int32      `json:"-"`
}

func (m *Pong) Reset()         { *m = Pong{} }
func (m *Pong) String() string { return proto.CompactTextString(m) }
func (*Pong) ProtoMessage()    {}
func (*Pong) Descriptor() ([]byte, []int) {
	return fileDescriptor_878fa4887b90140c, []int{10}
}

func (m *Pong) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Pong.Unmarshal(m, b)
}
func (m *Pong) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Pong.Marshal(b, m, deterministic)
}
func (m *Pong) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Pong.Merge(m, src)
}
func (m *Pong) XXX_Size() int {
	return xxx_messageInfo_Pong.Size(m)
}
func (m *Pong) XXX_DiscardUnknown() {
	xxx_messageInfo_Pong.DiscardUnknown(m)
}

var xxx_messageInfo_Pong proto.InternalMessageInfo

func (m *Pong) GetNonce() []byte {
	if m != nil {
		return m.Nonce
	}
	return nil
}

func (m *Pong) GetSignature() *Signature {
	if m != nil {
		return m.Signature
	}
	return nil
}

type Test struct {
	Nums                 []int32  `protobuf:"varint,1,rep,packed,name=nums,proto3" json:"nums,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Test) Reset()         { *m = Test{} }
func (m *Test) String() string { return proto.CompactTextString(m) }
func (*Test) ProtoMessage()    {}
func (*Test) Descriptor() ([]byte, []int) {
	return fileDescriptor_878fa4887b90140c, []int{11}
}

func (m *Test) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Test.Unmarshal(m, b)
}
func (m *Test) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Test.Marshal(b, m, deterministic)
}
func (m *Test) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Test.Merge(m, src)
}
func (m *Test) XXX_Size() int {
	return xxx_messageInfo_Test.Size(m)
}
func (m *Test) XXX_DiscardUnknown() {
	xxx_messageInfo_Test.DiscardUnknown(m)
}

var xxx_messageInfo_Test proto.InternalMessageInfo

func (m *Test) GetNums() []int32 {
	if m != nil {
		return m.Nums
	}
	return nil
}

func init() {
	proto.RegisterType((*State)(nil), "proto.State")
	proto.RegisterMapType((map[string]uint64)(nil), "proto.State.ExistingHostsEntry")
	proto.RegisterType((*Msg)(nil), "proto.Msg")
	proto.RegisterType((*MsgResponse)(nil), "proto.MsgResponse")
	proto.RegisterType((*StateResponse)(nil), "proto.StateResponse")
	proto.RegisterType((*Certificate)(nil), "proto.Certificate")
	proto.RegisterType((*Accusation)(nil), "proto.Accusation")
	proto.RegisterType((*Note)(nil), "proto.Note")
	proto.RegisterType((*Signature)(nil), "proto.Signature")
	proto.RegisterType((*Data)(nil), "proto.Data")
	proto.RegisterType((*Ping)(nil), "proto.Ping")
	proto.RegisterType((*Pong)(nil), "proto.Pong")
	proto.RegisterType((*Test)(nil), "proto.Test")
}

func init() { proto.RegisterFile("gossip.proto", fileDescriptor_878fa4887b90140c) }

var fileDescriptor_878fa4887b90140c = []byte{
	// 556 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x94, 0x54, 0xd1, 0x6e, 0xd3, 0x3c,
	0x14, 0xfe, 0x9d, 0x26, 0x9d, 0x7a, 0x92, 0x4e, 0xfb, 0xad, 0x5d, 0x44, 0x15, 0xd2, 0x4a, 0x24,
	0x20, 0x37, 0x54, 0x53, 0x27, 0x10, 0x42, 0x42, 0x02, 0x41, 0x05, 0x17, 0x74, 0x9a, 0x3c, 0x5e,
	0xc0, 0xa4, 0x87, 0x60, 0x6d, 0xb5, 0x23, 0xdb, 0x65, 0xdb, 0x93, 0x70, 0xcb, 0xdb, 0xf0, 0x18,
	0xbc, 0x0a, 0xb2, 0x93, 0xb4, 0xe9, 0x28, 0x4c, 0xbb, 0xea, 0xf9, 0x7c, 0xbe, 0x73, 0xce, 0xe7,
	0x2f, 0xa7, 0x86, 0xa4, 0x54, 0xc6, 0x88, 0x6a, 0x52, 0x69, 0x65, 0x15, 0x8d, 0xfc, 0x4f, 0xf6,
	0x8b, 0x40, 0x74, 0x6e, 0xb9, 0x45, 0x3a, 0x83, 0x21, 0x5e, 0x0b, 0x63, 0x85, 0x2c, 0x3f, 0x28,
	0x63, 0x4d, 0x4a, 0xc6, 0xbd, 0x3c, 0x9e, 0x1e, 0xd5, 0xfc, 0x89, 0x27, 0x4d, 0x66, 0x5d, 0xc6,
	0x4c, 0x5a, 0x7d, 0xc3, 0xb6, 0xab, 0xe8, 0x23, 0xd8, 0x53, 0x57, 0xf2, 0x54, 0x59, 0x4c, 0x83,
	0x31, 0xc9, 0xe3, 0x69, 0xdc, 0x34, 0x70, 0x47, 0xac, 0xcd, 0xd1, 0xc7, 0xb0, 0x8f, 0xd7, 0x16,
	0xb5, 0xe4, 0x97, 0xef, 0xbd, 0xac, 0xb4, 0x37, 0x26, 0x79, 0xc2, 0x6e, 0x9d, 0x8e, 0x5e, 0x03,
	0xfd, 0x73, 0x26, 0x3d, 0x80, 0xde, 0x05, 0xde, 0xa4, 0x64, 0x4c, 0xf2, 0x01, 0x73, 0x21, 0x3d,
	0x84, 0xe8, 0x1b, 0xbf, 0x5c, 0xd5, 0x43, 0x43, 0x56, 0x83, 0x97, 0xc1, 0x0b, 0x92, 0x3d, 0x83,
	0xde, 0xdc, 0x94, 0x34, 0x85, 0xbd, 0x42, 0x49, 0x8b, 0xd2, 0xfa, 0xb2, 0x84, 0xb5, 0xd0, 0x95,
	0xa2, 0xd6, 0x4a, 0xfb, 0xd2, 0x01, 0xab, 0x41, 0xf6, 0x0a, 0xe2, 0xb9, 0x29, 0x19, 0x9a, 0x4a,
	0x49, 0x83, 0xf7, 0x2e, 0xff, 0x49, 0x60, 0xe8, 0x2d, 0x5b, 0x77, 0x78, 0x0e, 0x49, 0x81, 0xda,
	0x8a, 0x2f, 0xa2, 0xe0, 0x16, 0x5b, 0x7b, 0x69, 0xe3, 0xce, 0xdb, 0x4d, 0x8a, 0x6d, 0xf1, 0xe8,
	0x43, 0x88, 0xa4, 0x72, 0x05, 0x81, 0x2f, 0xd8, 0xb2, 0xb3, 0xce, 0xd0, 0x13, 0x88, 0x79, 0x51,
	0xac, 0x0c, 0xb7, 0x42, 0x49, 0x93, 0xf6, 0x3c, 0xf1, 0xff, 0x86, 0xf8, 0x66, 0x9d, 0x61, 0x5d,
	0xd6, 0x8e, 0x2f, 0x10, 0xee, 0xfa, 0x02, 0xd9, 0x11, 0xc4, 0x1d, 0x71, 0xce, 0x7a, 0xcd, 0xaf,
	0x1a, 0x13, 0x5c, 0x98, 0xfd, 0x20, 0x00, 0x9b, 0x21, 0xde, 0x8f, 0x4a, 0x15, 0x5f, 0x3d, 0x25,
	0x64, 0x35, 0x70, 0xfe, 0xf9, 0xe1, 0x58, 0xfb, 0x94, 0xb0, 0x16, 0x6e, 0x32, 0x8b, 0x66, 0x05,
	0x5a, 0x48, 0x27, 0x30, 0x30, 0xa2, 0x94, 0xdc, 0xae, 0x34, 0x7a, 0x71, 0xf1, 0xf4, 0xa0, 0xdd,
	0xc6, 0xf6, 0x9c, 0x6d, 0x28, 0xae, 0x93, 0x16, 0xb2, 0x3c, 0x5d, 0x2d, 0xd3, 0x68, 0x4c, 0xf2,
	0x21, 0x6b, 0x61, 0x56, 0x41, 0xe8, 0xb7, 0x6e, 0xb7, 0xb6, 0x7d, 0x08, 0xc4, 0xa2, 0x91, 0x15,
	0x88, 0x05, 0xa5, 0x10, 0x2e, 0xb9, 0xb9, 0xf0, 0x72, 0x86, 0xcc, 0xc7, 0xf7, 0xd5, 0x92, 0x3d,
	0x81, 0xc1, 0xfa, 0x9c, 0x26, 0x40, 0x74, 0xe3, 0x18, 0xd1, 0x0e, 0x99, 0x66, 0x1a, 0x31, 0xd9,
	0x31, 0x84, 0xef, 0xb8, 0xe5, 0xff, 0x58, 0xb0, 0x5b, 0xf2, 0xb2, 0x07, 0x10, 0x9e, 0x09, 0x59,
	0xba, 0xcb, 0x48, 0x25, 0x0b, 0x6c, 0xf8, 0x35, 0xc8, 0x3e, 0x42, 0x78, 0xa6, 0xfe, 0x96, 0xdd,
	0xbe, 0x46, 0x70, 0xf7, 0x35, 0x46, 0x10, 0x7e, 0x42, 0x63, 0x9d, 0x25, 0x72, 0xb5, 0xac, 0x97,
	0x36, 0x62, 0x3e, 0x9e, 0x7e, 0x27, 0xd0, 0xaf, 0x9f, 0x14, 0x3a, 0x81, 0xfe, 0x79, 0xa5, 0x91,
	0x2f, 0x68, 0xd2, 0x7d, 0x2e, 0x46, 0x87, 0x5d, 0xd4, 0xfe, 0x13, 0xb2, 0xff, 0xe8, 0x53, 0x18,
	0xcc, 0xd1, 0x18, 0x94, 0x25, 0x6a, 0x0a, 0x0d, 0x69, 0x6e, 0xca, 0x11, 0xdd, 0xc4, 0x1d, 0xba,
	0x6b, 0x6f, 0x35, 0xf2, 0xe5, 0xdd, 0xdc, 0x9c, 0x1c, 0x93, 0xcf, 0x7d, 0x9f, 0x38, 0xf9, 0x1d,
	0x00, 0x00, 0xff, 0xff, 0xac, 0x7d, 0x8f, 0x9f, 0xf2, 0x04, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// GossipClient is the client API for Gossip service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type GossipClient interface {
	Spread(ctx context.Context, in *State, opts ...grpc.CallOption) (*StateResponse, error)
	Messenger(ctx context.Context, in *Msg, opts ...grpc.CallOption) (*MsgResponse, error)
	Stream(ctx context.Context, opts ...grpc.CallOption) (Gossip_StreamClient, error)
}

type gossipClient struct {
	cc *grpc.ClientConn
}

func NewGossipClient(cc *grpc.ClientConn) GossipClient {
	return &gossipClient{cc}
}

func (c *gossipClient) Spread(ctx context.Context, in *State, opts ...grpc.CallOption) (*StateResponse, error) {
	out := new(StateResponse)
	err := c.cc.Invoke(ctx, "/proto.gossip/Spread", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *gossipClient) Messenger(ctx context.Context, in *Msg, opts ...grpc.CallOption) (*MsgResponse, error) {
	out := new(MsgResponse)
	err := c.cc.Invoke(ctx, "/proto.gossip/Messenger", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *gossipClient) Stream(ctx context.Context, opts ...grpc.CallOption) (Gossip_StreamClient, error) {
	stream, err := c.cc.NewStream(ctx, &_Gossip_serviceDesc.Streams[0], "/proto.gossip/Stream", opts...)
	if err != nil {
		return nil, err
	}
	x := &gossipStreamClient{stream}
	return x, nil
}

type Gossip_StreamClient interface {
	Send(*Msg) error
	Recv() (*MsgResponse, error)
	grpc.ClientStream
}

type gossipStreamClient struct {
	grpc.ClientStream
}

func (x *gossipStreamClient) Send(m *Msg) error {
	return x.ClientStream.SendMsg(m)
}

func (x *gossipStreamClient) Recv() (*MsgResponse, error) {
	m := new(MsgResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// GossipServer is the server API for Gossip service.
type GossipServer interface {
	Spread(context.Context, *State) (*StateResponse, error)
	Messenger(context.Context, *Msg) (*MsgResponse, error)
	Stream(Gossip_StreamServer) error
}

// UnimplementedGossipServer can be embedded to have forward compatible implementations.
type UnimplementedGossipServer struct {
}

func (*UnimplementedGossipServer) Spread(ctx context.Context, req *State) (*StateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Spread not implemented")
}
func (*UnimplementedGossipServer) Messenger(ctx context.Context, req *Msg) (*MsgResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Messenger not implemented")
}
func (*UnimplementedGossipServer) Stream(srv Gossip_StreamServer) error {
	return status.Errorf(codes.Unimplemented, "method Stream not implemented")
}

func RegisterGossipServer(s *grpc.Server, srv GossipServer) {
	s.RegisterService(&_Gossip_serviceDesc, srv)
}

func _Gossip_Spread_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(State)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GossipServer).Spread(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/proto.gossip/Spread",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GossipServer).Spread(ctx, req.(*State))
	}
	return interceptor(ctx, in, info, handler)
}

func _Gossip_Messenger_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Msg)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GossipServer).Messenger(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/proto.gossip/Messenger",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GossipServer).Messenger(ctx, req.(*Msg))
	}
	return interceptor(ctx, in, info, handler)
}

func _Gossip_Stream_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(GossipServer).Stream(&gossipStreamServer{stream})
}

type Gossip_StreamServer interface {
	Send(*MsgResponse) error
	Recv() (*Msg, error)
	grpc.ServerStream
}

type gossipStreamServer struct {
	grpc.ServerStream
}

func (x *gossipStreamServer) Send(m *MsgResponse) error {
	return x.ServerStream.SendMsg(m)
}

func (x *gossipStreamServer) Recv() (*Msg, error) {
	m := new(Msg)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

var _Gossip_serviceDesc = grpc.ServiceDesc{
	ServiceName: "proto.gossip",
	HandlerType: (*GossipServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Spread",
			Handler:    _Gossip_Spread_Handler,
		},
		{
			MethodName: "Messenger",
			Handler:    _Gossip_Messenger_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Stream",
			Handler:       _Gossip_Stream_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "gossip.proto",
}
