package graph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapPICToTypes(t *testing.T) {
	tests := []struct {
		name    string
		picture string
		usage   string
		want    TypeMapping
	}{
		{
			name:    "integer display 9(5)",
			picture: "PIC 9(5)",
			usage:   "",
			want:    TypeMapping{JavaType: "int", SQLType: "INTEGER", StorageType: "DISPLAY", ByteLength: 5},
		},
		{
			name:    "signed decimal display S9(7)V99",
			picture: "S9(7)V99",
			usage:   "",
			want:    TypeMapping{JavaType: "BigDecimal", SQLType: "DECIMAL(9,2)", StorageType: "DISPLAY", ByteLength: 10},
		},
		{
			name:    "alphanumeric X(30)",
			picture: "PIC X(30)",
			usage:   "",
			want:    TypeMapping{JavaType: "String", SQLType: "VARCHAR(30)", StorageType: "DISPLAY", ByteLength: 30},
		},
		{
			name:    "packed decimal S9(9) COMP-3",
			picture: "S9(9)",
			usage:   "COMP-3",
			want:    TypeMapping{JavaType: "BigDecimal", SQLType: "DECIMAL(9,0)", StorageType: "PACKED_DECIMAL", ByteLength: 5},
		},
		{
			name:    "packed decimal with decimals S9(7)V9(2) COMP-3",
			picture: "S9(7)V9(2)",
			usage:   "COMP-3",
			want:    TypeMapping{JavaType: "BigDecimal", SQLType: "DECIMAL(9,2)", StorageType: "PACKED_DECIMAL", ByteLength: 5},
		},
		{
			name:    "binary small S9(4) COMP",
			picture: "S9(4)",
			usage:   "COMP",
			want:    TypeMapping{JavaType: "short", SQLType: "SMALLINT", StorageType: "BINARY", ByteLength: 2},
		},
		{
			name:    "binary int S9(9) COMP",
			picture: "S9(9)",
			usage:   "COMP",
			want:    TypeMapping{JavaType: "int", SQLType: "INTEGER", StorageType: "BINARY", ByteLength: 4},
		},
		{
			name:    "binary long 9(18) COMP",
			picture: "9(18)",
			usage:   "COMP",
			want:    TypeMapping{JavaType: "long", SQLType: "BIGINT", StorageType: "BINARY", ByteLength: 8},
		},
		{
			name:    "alphabetic A(10)",
			picture: "A(10)",
			usage:   "",
			want:    TypeMapping{JavaType: "String", SQLType: "VARCHAR(10)", StorageType: "DISPLAY", ByteLength: 10},
		},
		{
			name:    "expanded 999V99",
			picture: "999V99",
			usage:   "",
			want:    TypeMapping{JavaType: "BigDecimal", SQLType: "DECIMAL(5,2)", StorageType: "DISPLAY", ByteLength: 5},
		},
		{
			name:    "long display 9(12)",
			picture: "9(12)",
			usage:   "",
			want:    TypeMapping{JavaType: "long", SQLType: "BIGINT", StorageType: "DISPLAY", ByteLength: 12},
		},
		{
			name:    "empty picture",
			picture: "",
			usage:   "",
			want:    TypeMapping{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapPICToTypes(tt.picture, tt.usage)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExpandPIC(t *testing.T) {
	tests := []struct {
		pic       string
		intDigits int
		decDigits int
		signed    bool
		category  string
	}{
		{"9(5)", 5, 0, false, "numeric"},
		{"S9(7)V99", 7, 2, true, "numeric"},
		{"X(30)", 30, 0, false, "alphanumeric"},
		{"999V99", 3, 2, false, "numeric"},
		{"A(10)", 10, 0, false, "alphabetic"},
		{"S9(4)", 4, 0, true, "numeric"},
		{"PIC 9(5)", 5, 0, false, "numeric"},
	}

	for _, tt := range tests {
		t.Run(tt.pic, func(t *testing.T) {
			intD, decD, signed, cat := expandPIC(tt.pic)
			assert.Equal(t, tt.intDigits, intD, "intDigits")
			assert.Equal(t, tt.decDigits, decD, "decDigits")
			assert.Equal(t, tt.signed, signed, "signed")
			assert.Equal(t, tt.category, cat, "category")
		})
	}
}
