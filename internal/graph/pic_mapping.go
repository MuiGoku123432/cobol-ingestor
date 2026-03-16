package graph

import (
	"regexp"
	"strconv"
	"strings"
)

// TypeMapping holds the Java, SQL, and storage type mappings for a COBOL PIC clause.
type TypeMapping struct {
	JavaType    string `json:"javaType"`
	SQLType     string `json:"sqlType"`
	StorageType string `json:"storageType"`
	ByteLength  int    `json:"byteLength"`
}

// MapPICToTypes maps a COBOL PIC clause and USAGE to Java/SQL/storage types.
func MapPICToTypes(picture, usage string) TypeMapping {
	if picture == "" {
		return TypeMapping{}
	}

	intDigits, decDigits, signed, category := expandPIC(picture)
	usage = strings.ToUpper(strings.TrimSpace(usage))

	totalDigits := intDigits + decDigits

	switch {
	case category == "alphanumeric" || category == "alphabetic":
		return TypeMapping{
			JavaType:    "String",
			SQLType:     "VARCHAR(" + strconv.Itoa(intDigits) + ")",
			StorageType: "DISPLAY",
			ByteLength:  intDigits,
		}

	case category == "numeric" && (usage == "COMP-3" || usage == "PACKED-DECIMAL"):
		size := totalDigits
		if signed {
			size++
		}
		byteLen := (size + 1) / 2
		if decDigits > 0 {
			return TypeMapping{
				JavaType:    "BigDecimal",
				SQLType:     "DECIMAL(" + strconv.Itoa(totalDigits) + "," + strconv.Itoa(decDigits) + ")",
				StorageType: "PACKED_DECIMAL",
				ByteLength:  byteLen,
			}
		}
		return TypeMapping{
			JavaType:    "BigDecimal",
			SQLType:     "DECIMAL(" + strconv.Itoa(totalDigits) + ",0)",
			StorageType: "PACKED_DECIMAL",
			ByteLength:  byteLen,
		}

	case category == "numeric" && (usage == "COMP" || usage == "BINARY" || usage == "COMP-4"):
		if totalDigits <= 4 {
			return TypeMapping{
				JavaType:    "short",
				SQLType:     "SMALLINT",
				StorageType: "BINARY",
				ByteLength:  2,
			}
		}
		if totalDigits <= 9 {
			return TypeMapping{
				JavaType:    "int",
				SQLType:     "INTEGER",
				StorageType: "BINARY",
				ByteLength:  4,
			}
		}
		return TypeMapping{
			JavaType:    "long",
			SQLType:     "BIGINT",
			StorageType: "BINARY",
			ByteLength:  8,
		}

	case category == "numeric":
		// DISPLAY numeric
		byteLen := totalDigits
		if signed {
			byteLen++
		}
		if decDigits > 0 {
			return TypeMapping{
				JavaType:    "BigDecimal",
				SQLType:     "DECIMAL(" + strconv.Itoa(totalDigits) + "," + strconv.Itoa(decDigits) + ")",
				StorageType: "DISPLAY",
				ByteLength:  byteLen,
			}
		}
		if totalDigits <= 9 {
			return TypeMapping{
				JavaType:    "int",
				SQLType:     "INTEGER",
				StorageType: "DISPLAY",
				ByteLength:  byteLen,
			}
		}
		return TypeMapping{
			JavaType:    "long",
			SQLType:     "BIGINT",
			StorageType: "DISPLAY",
			ByteLength:  byteLen,
		}
	}

	return TypeMapping{}
}

// repeatRegex matches patterns like 9(5), X(30), A(10)
var repeatRegex = regexp.MustCompile(`([9XASV])\((\d+)\)`)

// expandPIC parses a COBOL PIC clause and returns the integer digits, decimal digits,
// whether it's signed, and the category (numeric, alphanumeric, alphabetic).
func expandPIC(pic string) (intDigits, decDigits int, signed bool, category string) {
	// Normalize: remove "PIC " or "PIC(" prefix, uppercase
	p := strings.ToUpper(strings.TrimSpace(pic))
	p = strings.TrimPrefix(p, "PIC ")
	p = strings.TrimPrefix(p, "PIC")
	p = strings.TrimSpace(p)

	// Check for sign
	if strings.HasPrefix(p, "S") {
		signed = true
		p = p[1:]
	}

	// Expand shorthand: 9(5) -> 99999, X(30) -> XXX...
	expanded := repeatRegex.ReplaceAllStringFunc(p, func(match string) string {
		sub := repeatRegex.FindStringSubmatch(match)
		if len(sub) == 3 {
			ch := sub[1]
			n, _ := strconv.Atoi(sub[2])
			return strings.Repeat(ch, n)
		}
		return match
	})

	// Classify based on content
	hasX := strings.ContainsAny(expanded, "X")
	hasA := strings.ContainsAny(expanded, "A")
	has9 := strings.ContainsAny(expanded, "9")

	if hasX {
		category = "alphanumeric"
		intDigits = strings.Count(expanded, "X")
		return
	}
	if hasA && !has9 {
		category = "alphabetic"
		intDigits = strings.Count(expanded, "A")
		return
	}

	// Numeric: count digits before and after V (implied decimal)
	category = "numeric"
	parts := strings.SplitN(expanded, "V", 2)
	intDigits = strings.Count(parts[0], "9")
	if len(parts) > 1 {
		decDigits = strings.Count(parts[1], "9")
	}
	return
}
