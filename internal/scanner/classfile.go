package scanner

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// ClassFileInfo holds parsed metadata from a Java .class file.
type ClassFileInfo struct {
	ClassName  string
	SuperClass string
	Interfaces []string
	Fields     []ClassField
	Methods    []ClassMethod
	SourceFile string
}

// ClassField represents a field in a Java class file.
type ClassField struct {
	Name        string
	Descriptor  string
	AccessFlags uint16
}

// ClassMethod represents a method in a Java class file.
type ClassMethod struct {
	Name        string
	Descriptor  string
	AccessFlags uint16
}

const (
	classMagic = 0xCAFEBABE

	// Access flag constants
	accPublic    = 0x0001
	accPrivate   = 0x0002
	accProtected = 0x0004
	accStatic    = 0x0008
	accFinal     = 0x0010
	accAbstract  = 0x0400

	// Constant pool tag values
	cpUTF8               = 1
	cpInteger            = 3
	cpFloat              = 4
	cpLong               = 5
	cpDouble             = 6
	cpClass              = 7
	cpString             = 8
	cpFieldRef           = 9
	cpMethodRef          = 10
	cpInterfaceMethodRef = 11
	cpNameAndType        = 12
	cpMethodHandle       = 15
	cpMethodType         = 16
	cpDynamic            = 17
	cpInvokeDynamic      = 18
	cpModule             = 19
	cpPackage            = 20
)

// ParseClassFile reads a Java class file's constant pool to extract class metadata.
func ParseClassFile(data []byte) (*ClassFileInfo, error) {
	if len(data) < 10 {
		return nil, fmt.Errorf("class file too small (%d bytes)", len(data))
	}

	magic := binary.BigEndian.Uint32(data[0:4])
	if magic != classMagic {
		return nil, fmt.Errorf("invalid class magic: 0x%08X", magic)
	}

	// Skip minor/major version (4 bytes)
	cpCount := int(binary.BigEndian.Uint16(data[8:10]))
	if cpCount < 1 {
		return nil, fmt.Errorf("invalid constant pool count: %d", cpCount)
	}

	// Parse constant pool
	utf8s := make(map[int]string)       // index -> string
	classRefs := make(map[int]int)      // index -> utf8 index
	nameAndTypes := make(map[int][2]int) // index -> [name_idx, desc_idx]

	pos := 10
	for i := 1; i < cpCount; i++ {
		if pos >= len(data) {
			return nil, fmt.Errorf("truncated constant pool at index %d", i)
		}
		tag := data[pos]
		pos++

		switch tag {
		case cpUTF8:
			if pos+2 > len(data) {
				return nil, fmt.Errorf("truncated UTF8 entry at index %d", i)
			}
			length := int(binary.BigEndian.Uint16(data[pos : pos+2]))
			pos += 2
			if pos+length > len(data) {
				return nil, fmt.Errorf("truncated UTF8 data at index %d", i)
			}
			utf8s[i] = string(data[pos : pos+length])
			pos += length

		case cpInteger, cpFloat:
			pos += 4

		case cpLong, cpDouble:
			pos += 8
			i++ // longs and doubles take two slots

		case cpClass, cpString, cpMethodType, cpModule, cpPackage:
			if pos+2 > len(data) {
				return nil, fmt.Errorf("truncated entry at index %d", i)
			}
			if tag == cpClass {
				classRefs[i] = int(binary.BigEndian.Uint16(data[pos : pos+2]))
			}
			pos += 2

		case cpFieldRef, cpMethodRef, cpInterfaceMethodRef:
			pos += 4

		case cpNameAndType:
			if pos+4 > len(data) {
				return nil, fmt.Errorf("truncated NameAndType at index %d", i)
			}
			nameIdx := int(binary.BigEndian.Uint16(data[pos : pos+2]))
			descIdx := int(binary.BigEndian.Uint16(data[pos+2 : pos+4]))
			nameAndTypes[i] = [2]int{nameIdx, descIdx}
			pos += 4

		case cpMethodHandle:
			pos += 3

		case cpDynamic, cpInvokeDynamic:
			pos += 4

		default:
			return nil, fmt.Errorf("unknown constant pool tag %d at index %d", tag, i)
		}
	}

	// Helper to resolve class name from class_info index
	resolveClass := func(idx int) string {
		if nameIdx, ok := classRefs[idx]; ok {
			if name, ok := utf8s[nameIdx]; ok {
				return strings.ReplaceAll(name, "/", ".")
			}
		}
		return ""
	}

	// Read access flags, this class, super class
	if pos+6 > len(data) {
		return nil, fmt.Errorf("truncated class info")
	}
	// accessFlags := binary.BigEndian.Uint16(data[pos : pos+2])
	thisClass := int(binary.BigEndian.Uint16(data[pos+2 : pos+4]))
	superClass := int(binary.BigEndian.Uint16(data[pos+4 : pos+6]))
	pos += 6

	info := &ClassFileInfo{
		ClassName:  resolveClass(thisClass),
		SuperClass: resolveClass(superClass),
	}

	// Read interfaces
	if pos+2 > len(data) {
		return nil, fmt.Errorf("truncated interfaces count")
	}
	ifaceCount := int(binary.BigEndian.Uint16(data[pos : pos+2]))
	pos += 2
	for j := 0; j < ifaceCount; j++ {
		if pos+2 > len(data) {
			return nil, fmt.Errorf("truncated interface entry")
		}
		ifaceIdx := int(binary.BigEndian.Uint16(data[pos : pos+2]))
		pos += 2
		if name := resolveClass(ifaceIdx); name != "" {
			info.Interfaces = append(info.Interfaces, name)
		}
	}

	// Read fields
	if pos+2 > len(data) {
		return info, nil // partial parse is OK
	}
	fieldCount := int(binary.BigEndian.Uint16(data[pos : pos+2]))
	pos += 2
	for j := 0; j < fieldCount; j++ {
		if pos+8 > len(data) {
			return info, nil
		}
		flags := binary.BigEndian.Uint16(data[pos : pos+2])
		nameIdx := int(binary.BigEndian.Uint16(data[pos+2 : pos+4]))
		descIdx := int(binary.BigEndian.Uint16(data[pos+4 : pos+6]))
		attrCount := int(binary.BigEndian.Uint16(data[pos+6 : pos+8]))
		pos += 8

		info.Fields = append(info.Fields, ClassField{
			Name:        utf8s[nameIdx],
			Descriptor:  utf8s[descIdx],
			AccessFlags: flags,
		})

		// Skip attributes
		for k := 0; k < attrCount; k++ {
			if pos+6 > len(data) {
				return info, nil
			}
			attrLen := int(binary.BigEndian.Uint32(data[pos+2 : pos+6]))
			pos += 6 + attrLen
		}
	}

	// Read methods
	if pos+2 > len(data) {
		return info, nil
	}
	methodCount := int(binary.BigEndian.Uint16(data[pos : pos+2]))
	pos += 2
	for j := 0; j < methodCount; j++ {
		if pos+8 > len(data) {
			return info, nil
		}
		flags := binary.BigEndian.Uint16(data[pos : pos+2])
		nameIdx := int(binary.BigEndian.Uint16(data[pos+2 : pos+4]))
		descIdx := int(binary.BigEndian.Uint16(data[pos+4 : pos+6]))
		attrCount := int(binary.BigEndian.Uint16(data[pos+6 : pos+8]))
		pos += 8

		info.Methods = append(info.Methods, ClassMethod{
			Name:        utf8s[nameIdx],
			Descriptor:  utf8s[descIdx],
			AccessFlags: flags,
		})

		// Skip attributes
		for k := 0; k < attrCount; k++ {
			if pos+6 > len(data) {
				return info, nil
			}
			attrLen := int(binary.BigEndian.Uint32(data[pos+2 : pos+6]))
			pos += 6 + attrLen
		}
	}

	// Try to find SourceFile attribute
	if pos+2 <= len(data) {
		attrCount := int(binary.BigEndian.Uint16(data[pos : pos+2]))
		pos += 2
		for j := 0; j < attrCount; j++ {
			if pos+6 > len(data) {
				break
			}
			nameIdx := int(binary.BigEndian.Uint16(data[pos : pos+2]))
			attrLen := int(binary.BigEndian.Uint32(data[pos+2 : pos+6]))
			pos += 6

			if utf8s[nameIdx] == "SourceFile" && attrLen == 2 && pos+2 <= len(data) {
				srcIdx := int(binary.BigEndian.Uint16(data[pos : pos+2]))
				info.SourceFile = utf8s[srcIdx]
			}
			pos += attrLen
		}
	}

	return info, nil
}

// FormatAsText renders the class file info as human-readable text suitable for LLM analysis.
func (c *ClassFileInfo) FormatAsText() string {
	var b strings.Builder

	fmt.Fprintf(&b, "// Disassembled class file (Go-native parser)\n")
	if c.SourceFile != "" {
		fmt.Fprintf(&b, "// Source: %s\n", c.SourceFile)
	}
	fmt.Fprintf(&b, "\n")

	// Class declaration
	fmt.Fprintf(&b, "class %s", c.ClassName)
	if c.SuperClass != "" && c.SuperClass != "java.lang.Object" {
		fmt.Fprintf(&b, " extends %s", c.SuperClass)
	}
	if len(c.Interfaces) > 0 {
		fmt.Fprintf(&b, " implements %s", strings.Join(c.Interfaces, ", "))
	}
	fmt.Fprintf(&b, " {\n\n")

	// Fields
	for _, f := range c.Fields {
		fmt.Fprintf(&b, "  %s%s %s;\n", formatAccessFlags(f.AccessFlags), formatDescriptor(f.Descriptor), f.Name)
	}
	if len(c.Fields) > 0 {
		fmt.Fprintf(&b, "\n")
	}

	// Methods
	for _, m := range c.Methods {
		ret, params := parseMethodDescriptor(m.Descriptor)
		fmt.Fprintf(&b, "  %s%s %s(%s);\n", formatAccessFlags(m.AccessFlags), ret, m.Name, params)
	}

	fmt.Fprintf(&b, "}\n")
	return b.String()
}

// formatAccessFlags converts a bitmask to a Java-like modifier string.
func formatAccessFlags(flags uint16) string {
	var parts []string
	if flags&accPublic != 0 {
		parts = append(parts, "public")
	}
	if flags&accPrivate != 0 {
		parts = append(parts, "private")
	}
	if flags&accProtected != 0 {
		parts = append(parts, "protected")
	}
	if flags&accStatic != 0 {
		parts = append(parts, "static")
	}
	if flags&accFinal != 0 {
		parts = append(parts, "final")
	}
	if flags&accAbstract != 0 {
		parts = append(parts, "abstract")
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ") + " "
}

// formatDescriptor converts a JVM type descriptor to a readable type name.
func formatDescriptor(desc string) string {
	t, _ := parseType(desc)
	return t
}

// parseType parses one JVM type descriptor and returns (type_name, remaining_string).
func parseType(desc string) (string, string) {
	if len(desc) == 0 {
		return "void", ""
	}
	switch desc[0] {
	case 'V':
		return "void", desc[1:]
	case 'Z':
		return "boolean", desc[1:]
	case 'B':
		return "byte", desc[1:]
	case 'C':
		return "char", desc[1:]
	case 'S':
		return "short", desc[1:]
	case 'I':
		return "int", desc[1:]
	case 'J':
		return "long", desc[1:]
	case 'F':
		return "float", desc[1:]
	case 'D':
		return "double", desc[1:]
	case '[':
		inner, rest := parseType(desc[1:])
		return inner + "[]", rest
	case 'L':
		idx := strings.IndexByte(desc, ';')
		if idx < 0 {
			return desc, ""
		}
		name := strings.ReplaceAll(desc[1:idx], "/", ".")
		// Use simple name
		if dotIdx := strings.LastIndex(name, "."); dotIdx >= 0 {
			name = name[dotIdx+1:]
		}
		return name, desc[idx+1:]
	default:
		return string(desc[0]), desc[1:]
	}
}

// parseMethodDescriptor converts "(Ljava/lang/String;I)V" to return type and params string.
func parseMethodDescriptor(desc string) (string, string) {
	if len(desc) == 0 || desc[0] != '(' {
		return "void", ""
	}

	inner := desc[1:]
	var params []string
	for len(inner) > 0 && inner[0] != ')' {
		t, rest := parseType(inner)
		params = append(params, t)
		inner = rest
	}

	ret := "void"
	if len(inner) > 1 {
		ret, _ = parseType(inner[1:])
	}

	return ret, strings.Join(params, ", ")
}
