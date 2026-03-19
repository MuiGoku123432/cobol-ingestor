package neo4j

import "testing"

func TestScopedKey(t *testing.T) {
	tests := []struct {
		codebase string
		value    string
		want     string
	}{
		{"SYS_A", "CUSTMAIN", "SYS_A::CUSTMAIN"},
		{"default", "CUSTMAIN", "CUSTMAIN"},
		{"", "CUSTMAIN", "CUSTMAIN"},
		{"SYS_B", "PROG1.MAIN-PARA", "SYS_B::PROG1.MAIN-PARA"},
		{"SYS_A", "PROG1.05.WS-FIELD", "SYS_A::PROG1.05.WS-FIELD"},
		{"default", "", ""},
		{"SYS_A", "", "SYS_A::"},
	}

	for _, tt := range tests {
		got := ScopedKey(tt.codebase, tt.value)
		if got != tt.want {
			t.Errorf("ScopedKey(%q, %q) = %q, want %q", tt.codebase, tt.value, got, tt.want)
		}
	}
}

func TestIsSharedLabel(t *testing.T) {
	shared := []string{"Copybook", "File", "BusinessDomain", "DBTable", "ExternalDatabase", "ExternalDBTable"}
	for _, label := range shared {
		if !isSharedLabel(label) {
			t.Errorf("isSharedLabel(%q) = false, want true", label)
		}
	}

	scoped := []string{"Program", "Paragraph", "Section", "DataItem", "Condition", "Parameter",
		"SQLStatement", "CICSTransaction", "ExternalInterface", "JCLJob", "JCLStep", "DDCard"}
	for _, label := range scoped {
		if isSharedLabel(label) {
			t.Errorf("isSharedLabel(%q) = true, want false", label)
		}
	}
}
