package scanner

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildMinimalClassFile constructs a minimal valid Java .class file with
// a class name, superclass, one field, and one method.
func buildMinimalClassFile() []byte {
	// We'll build a constant pool manually.
	// Layout:
	//   CP#1: UTF8 "HelloWorld"
	//   CP#2: Class -> #1
	//   CP#3: UTF8 "java/lang/Object"
	//   CP#4: Class -> #3
	//   CP#5: UTF8 "message"          (field name)
	//   CP#6: UTF8 "Ljava/lang/String;" (field descriptor)
	//   CP#7: UTF8 "greet"            (method name)
	//   CP#8: UTF8 "(Ljava/lang/String;)V" (method descriptor)
	//   CP#9: UTF8 "Code"             (attribute name, needed for methods)
	//   CP#10: UTF8 "SourceFile"
	//   CP#11: UTF8 "HelloWorld.java"

	var data []byte

	// Magic
	data = binary.BigEndian.AppendUint32(data, 0xCAFEBABE)
	// Minor version
	data = binary.BigEndian.AppendUint16(data, 0)
	// Major version (Java 8 = 52)
	data = binary.BigEndian.AppendUint16(data, 52)

	// Constant pool count (11 entries + 1 = 12)
	data = binary.BigEndian.AppendUint16(data, 12)

	// Helper to write UTF8 constant
	writeUTF8 := func(s string) {
		data = append(data, 1) // CONSTANT_Utf8 tag
		data = binary.BigEndian.AppendUint16(data, uint16(len(s)))
		data = append(data, []byte(s)...)
	}
	// Helper to write Class constant
	writeClass := func(nameIdx uint16) {
		data = append(data, 7) // CONSTANT_Class tag
		data = binary.BigEndian.AppendUint16(data, nameIdx)
	}

	// CP#1
	writeUTF8("HelloWorld")
	// CP#2
	writeClass(1)
	// CP#3
	writeUTF8("java/lang/Object")
	// CP#4
	writeClass(3)
	// CP#5
	writeUTF8("message")
	// CP#6
	writeUTF8("Ljava/lang/String;")
	// CP#7
	writeUTF8("greet")
	// CP#8
	writeUTF8("(Ljava/lang/String;)V")
	// CP#9
	writeUTF8("Code")
	// CP#10
	writeUTF8("SourceFile")
	// CP#11
	writeUTF8("HelloWorld.java")

	// Access flags: public (0x0001)
	data = binary.BigEndian.AppendUint16(data, 0x0001)
	// This class: CP#2
	data = binary.BigEndian.AppendUint16(data, 2)
	// Super class: CP#4
	data = binary.BigEndian.AppendUint16(data, 4)

	// Interfaces count: 0
	data = binary.BigEndian.AppendUint16(data, 0)

	// Fields count: 1
	data = binary.BigEndian.AppendUint16(data, 1)
	// Field: private String message
	data = binary.BigEndian.AppendUint16(data, 0x0002) // private
	data = binary.BigEndian.AppendUint16(data, 5)      // name: "message"
	data = binary.BigEndian.AppendUint16(data, 6)      // descriptor: "Ljava/lang/String;"
	data = binary.BigEndian.AppendUint16(data, 0)      // attributes count: 0

	// Methods count: 1
	data = binary.BigEndian.AppendUint16(data, 1)
	// Method: public void greet(String)
	data = binary.BigEndian.AppendUint16(data, 0x0001) // public
	data = binary.BigEndian.AppendUint16(data, 7)      // name: "greet"
	data = binary.BigEndian.AppendUint16(data, 8)      // descriptor: "(Ljava/lang/String;)V"
	data = binary.BigEndian.AppendUint16(data, 0)      // attributes count: 0

	// Class attributes count: 1 (SourceFile)
	data = binary.BigEndian.AppendUint16(data, 1)
	// SourceFile attribute
	data = binary.BigEndian.AppendUint16(data, 10) // name: "SourceFile"
	data = binary.BigEndian.AppendUint32(data, 2)  // length: 2
	data = binary.BigEndian.AppendUint16(data, 11) // sourcefile: "HelloWorld.java"

	return data
}

func TestParseClassFile(t *testing.T) {
	data := buildMinimalClassFile()

	info, err := ParseClassFile(data)
	require.NoError(t, err)

	assert.Equal(t, "HelloWorld", info.ClassName)
	assert.Equal(t, "java.lang.Object", info.SuperClass)
	assert.Empty(t, info.Interfaces)

	require.Len(t, info.Fields, 1)
	assert.Equal(t, "message", info.Fields[0].Name)
	assert.Equal(t, "Ljava/lang/String;", info.Fields[0].Descriptor)
	assert.Equal(t, uint16(0x0002), info.Fields[0].AccessFlags) // private

	require.Len(t, info.Methods, 1)
	assert.Equal(t, "greet", info.Methods[0].Name)
	assert.Equal(t, "(Ljava/lang/String;)V", info.Methods[0].Descriptor)
	assert.Equal(t, uint16(0x0001), info.Methods[0].AccessFlags) // public

	assert.Equal(t, "HelloWorld.java", info.SourceFile)
}

func TestParseClassFile_InvalidMagic(t *testing.T) {
	data := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	_, err := ParseClassFile(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid class magic")
}

func TestParseClassFile_TooSmall(t *testing.T) {
	_, err := ParseClassFile([]byte{0xCA, 0xFE})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too small")
}

func TestFormatAsText(t *testing.T) {
	info := &ClassFileInfo{
		ClassName:  "com.example.Service",
		SuperClass: "com.example.BaseService",
		Interfaces: []string{"java.io.Serializable"},
		Fields: []ClassField{
			{Name: "count", Descriptor: "I", AccessFlags: accPrivate},
			{Name: "name", Descriptor: "Ljava/lang/String;", AccessFlags: accPublic},
		},
		Methods: []ClassMethod{
			{Name: "<init>", Descriptor: "()V", AccessFlags: accPublic},
			{Name: "process", Descriptor: "(Ljava/lang/String;I)Ljava/lang/String;", AccessFlags: accPublic},
		},
		SourceFile: "Service.java",
	}

	text := info.FormatAsText()

	assert.Contains(t, text, "class com.example.Service")
	assert.Contains(t, text, "extends com.example.BaseService")
	assert.Contains(t, text, "implements java.io.Serializable")
	assert.Contains(t, text, "private int count")
	assert.Contains(t, text, "public String name")
	assert.Contains(t, text, "public void <init>()")
	assert.Contains(t, text, "public String process(String, int)")
	assert.Contains(t, text, "Source: Service.java")
}

func TestFormatAsText_OmitsObjectSuperclass(t *testing.T) {
	info := &ClassFileInfo{
		ClassName:  "Simple",
		SuperClass: "java.lang.Object",
	}

	text := info.FormatAsText()
	assert.Contains(t, text, "class Simple {")
	assert.NotContains(t, text, "extends")
}

func TestParseMethodDescriptor(t *testing.T) {
	tests := []struct {
		desc       string
		wantRet    string
		wantParams string
	}{
		{"()V", "void", ""},
		{"(I)V", "void", "int"},
		{"(Ljava/lang/String;I)Ljava/lang/String;", "String", "String, int"},
		{"([B)V", "void", "byte[]"},
		{"(DD)D", "double", "double, double"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ret, params := parseMethodDescriptor(tt.desc)
			assert.Equal(t, tt.wantRet, ret)
			assert.Equal(t, tt.wantParams, params)
		})
	}
}

func TestClassFileRoundTrip(t *testing.T) {
	// Build a class file, parse it, format it, and verify the output is sensible
	data := buildMinimalClassFile()
	info, err := ParseClassFile(data)
	require.NoError(t, err)

	text := info.FormatAsText()
	// Should contain key structural elements
	assert.True(t, strings.Contains(text, "class HelloWorld"))
	assert.True(t, strings.Contains(text, "greet"))
	assert.True(t, strings.Contains(text, "message"))
}
