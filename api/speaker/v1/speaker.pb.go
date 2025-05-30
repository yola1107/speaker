// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        v3.6.1
// source: speaker/v1/speaker.proto

package v1

import (
	_ "google.golang.org/genproto/googleapis/api/annotations"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type GameCommand int32

const (
	GameCommand_Nothing      GameCommand = 0    //
	GameCommand_Ping         GameCommand = 1    //
	GameCommand_PushExample  GameCommand = 2    //push
	GameCommand_SayHelloReq  GameCommand = 1001 //
	GameCommand_SayHelloRsp  GameCommand = 1002
	GameCommand_SayHello2Req GameCommand = 1003
	GameCommand_SayHello2Rsp GameCommand = 1004
)

// Enum value maps for GameCommand.
var (
	GameCommand_name = map[int32]string{
		0:    "Nothing",
		1:    "Ping",
		2:    "PushExample",
		1001: "SayHelloReq",
		1002: "SayHelloRsp",
		1003: "SayHello2Req",
		1004: "SayHello2Rsp",
	}
	GameCommand_value = map[string]int32{
		"Nothing":      0,
		"Ping":         1,
		"PushExample":  2,
		"SayHelloReq":  1001,
		"SayHelloRsp":  1002,
		"SayHello2Req": 1003,
		"SayHello2Rsp": 1004,
	}
)

func (x GameCommand) Enum() *GameCommand {
	p := new(GameCommand)
	*p = x
	return p
}

func (x GameCommand) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (GameCommand) Descriptor() protoreflect.EnumDescriptor {
	return file_speaker_v1_speaker_proto_enumTypes[0].Descriptor()
}

func (GameCommand) Type() protoreflect.EnumType {
	return &file_speaker_v1_speaker_proto_enumTypes[0]
}

func (x GameCommand) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use GameCommand.Descriptor instead.
func (GameCommand) EnumDescriptor() ([]byte, []int) {
	return file_speaker_v1_speaker_proto_rawDescGZIP(), []int{0}
}

// The request message containing the user's name.
type HelloRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Name          string                 `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *HelloRequest) Reset() {
	*x = HelloRequest{}
	mi := &file_speaker_v1_speaker_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *HelloRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HelloRequest) ProtoMessage() {}

func (x *HelloRequest) ProtoReflect() protoreflect.Message {
	mi := &file_speaker_v1_speaker_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use HelloRequest.ProtoReflect.Descriptor instead.
func (*HelloRequest) Descriptor() ([]byte, []int) {
	return file_speaker_v1_speaker_proto_rawDescGZIP(), []int{0}
}

func (x *HelloRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

// The response message containing the greetings
type HelloReply struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Message       string                 `protobuf:"bytes,1,opt,name=message,proto3" json:"message,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *HelloReply) Reset() {
	*x = HelloReply{}
	mi := &file_speaker_v1_speaker_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *HelloReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HelloReply) ProtoMessage() {}

func (x *HelloReply) ProtoReflect() protoreflect.Message {
	mi := &file_speaker_v1_speaker_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use HelloReply.ProtoReflect.Descriptor instead.
func (*HelloReply) Descriptor() ([]byte, []int) {
	return file_speaker_v1_speaker_proto_rawDescGZIP(), []int{1}
}

func (x *HelloReply) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

// The request message containing the user's name.
type Hello2Request struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Name          string                 `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Hello2Request) Reset() {
	*x = Hello2Request{}
	mi := &file_speaker_v1_speaker_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Hello2Request) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Hello2Request) ProtoMessage() {}

func (x *Hello2Request) ProtoReflect() protoreflect.Message {
	mi := &file_speaker_v1_speaker_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Hello2Request.ProtoReflect.Descriptor instead.
func (*Hello2Request) Descriptor() ([]byte, []int) {
	return file_speaker_v1_speaker_proto_rawDescGZIP(), []int{2}
}

func (x *Hello2Request) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

// The response message containing the greetings
type Hello2Reply struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Message       string                 `protobuf:"bytes,1,opt,name=message,proto3" json:"message,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Hello2Reply) Reset() {
	*x = Hello2Reply{}
	mi := &file_speaker_v1_speaker_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Hello2Reply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Hello2Reply) ProtoMessage() {}

func (x *Hello2Reply) ProtoReflect() protoreflect.Message {
	mi := &file_speaker_v1_speaker_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Hello2Reply.ProtoReflect.Descriptor instead.
func (*Hello2Reply) Descriptor() ([]byte, []int) {
	return file_speaker_v1_speaker_proto_rawDescGZIP(), []int{3}
}

func (x *Hello2Reply) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

var File_speaker_v1_speaker_proto protoreflect.FileDescriptor

const file_speaker_v1_speaker_proto_rawDesc = "" +
	"\n" +
	"\x18speaker/v1/speaker.proto\x12\rhelloworld.v1\x1a\x1cgoogle/api/annotations.proto\"\"\n" +
	"\fHelloRequest\x12\x12\n" +
	"\x04name\x18\x01 \x01(\tR\x04name\"&\n" +
	"\n" +
	"HelloReply\x12\x18\n" +
	"\amessage\x18\x01 \x01(\tR\amessage\"#\n" +
	"\rHello2Request\x12\x12\n" +
	"\x04name\x18\x01 \x01(\tR\x04name\"'\n" +
	"\vHello2Reply\x12\x18\n" +
	"\amessage\x18\x01 \x01(\tR\amessage*\x7f\n" +
	"\vGameCommand\x12\v\n" +
	"\aNothing\x10\x00\x12\b\n" +
	"\x04Ping\x10\x01\x12\x0f\n" +
	"\vPushExample\x10\x02\x12\x10\n" +
	"\vSayHelloReq\x10\xe9\a\x12\x10\n" +
	"\vSayHelloRsp\x10\xea\a\x12\x11\n" +
	"\fSayHello2Req\x10\xeb\a\x12\x11\n" +
	"\fSayHello2Rsp\x10\xec\a2\xd8\x01\n" +
	"\aSpeaker\x12a\n" +
	"\vSayHelloReq\x12\x1b.helloworld.v1.HelloRequest\x1a\x19.helloworld.v1.HelloReply\"\x1a\x82\xd3\xe4\x93\x02\x14\x12\x12/helloworld/{name}\x12j\n" +
	"\fSayHello2Req\x12\x1c.helloworld.v1.Hello2Request\x1a\x1a.helloworld.v1.Hello2Reply\" \x82\xd3\xe4\x93\x02\x1a:\x01*\"\x15/greeter/SayHello2ReqBQ\n" +
	"\x1cdev.kratos.api.helloworld.v1B\x11HelloworldProtoV1P\x01Z\x1cspeaker/api/helloworld/v1;v1b\x06proto3"

var (
	file_speaker_v1_speaker_proto_rawDescOnce sync.Once
	file_speaker_v1_speaker_proto_rawDescData []byte
)

func file_speaker_v1_speaker_proto_rawDescGZIP() []byte {
	file_speaker_v1_speaker_proto_rawDescOnce.Do(func() {
		file_speaker_v1_speaker_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_speaker_v1_speaker_proto_rawDesc), len(file_speaker_v1_speaker_proto_rawDesc)))
	})
	return file_speaker_v1_speaker_proto_rawDescData
}

var file_speaker_v1_speaker_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_speaker_v1_speaker_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_speaker_v1_speaker_proto_goTypes = []any{
	(GameCommand)(0),      // 0: helloworld.v1.GameCommand
	(*HelloRequest)(nil),  // 1: helloworld.v1.HelloRequest
	(*HelloReply)(nil),    // 2: helloworld.v1.HelloReply
	(*Hello2Request)(nil), // 3: helloworld.v1.Hello2Request
	(*Hello2Reply)(nil),   // 4: helloworld.v1.Hello2Reply
}
var file_speaker_v1_speaker_proto_depIdxs = []int32{
	1, // 0: helloworld.v1.Speaker.SayHelloReq:input_type -> helloworld.v1.HelloRequest
	3, // 1: helloworld.v1.Speaker.SayHello2Req:input_type -> helloworld.v1.Hello2Request
	2, // 2: helloworld.v1.Speaker.SayHelloReq:output_type -> helloworld.v1.HelloReply
	4, // 3: helloworld.v1.Speaker.SayHello2Req:output_type -> helloworld.v1.Hello2Reply
	2, // [2:4] is the sub-list for method output_type
	0, // [0:2] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_speaker_v1_speaker_proto_init() }
func file_speaker_v1_speaker_proto_init() {
	if File_speaker_v1_speaker_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_speaker_v1_speaker_proto_rawDesc), len(file_speaker_v1_speaker_proto_rawDesc)),
			NumEnums:      1,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_speaker_v1_speaker_proto_goTypes,
		DependencyIndexes: file_speaker_v1_speaker_proto_depIdxs,
		EnumInfos:         file_speaker_v1_speaker_proto_enumTypes,
		MessageInfos:      file_speaker_v1_speaker_proto_msgTypes,
	}.Build()
	File_speaker_v1_speaker_proto = out.File
	file_speaker_v1_speaker_proto_goTypes = nil
	file_speaker_v1_speaker_proto_depIdxs = nil
}
