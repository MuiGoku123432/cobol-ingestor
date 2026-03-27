package static

import (
	"testing"

	"cobol-ingestor/internal/graph"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractDataHierarchy_Simple(t *testing.T) {
	data := `       DATA DIVISION.
       WORKING-STORAGE SECTION.
       01  WS-CUSTOMER-REC.
           05  WS-CUST-ID        PIC X(10).
           05  WS-CUST-NAME      PIC X(30).
           05  WS-CUST-ADDR.
               10  WS-STREET     PIC X(40).
               10  WS-CITY       PIC X(20).
       01  WS-FLAGS.
           05  WS-EOF-FLAG       PIC X(1).`

	rels := ExtractDataHierarchy(data, "CUSTPROG")
	require.NotEmpty(t, rels)

	// WS-CUST-ID should be child of WS-CUSTOMER-REC
	assertChildOf(t, rels, "CUSTPROG.05.WS-CUST-ID", "CUSTPROG.01.WS-CUSTOMER-REC")
	// WS-CUST-NAME should be child of WS-CUSTOMER-REC
	assertChildOf(t, rels, "CUSTPROG.05.WS-CUST-NAME", "CUSTPROG.01.WS-CUSTOMER-REC")
	// WS-CUST-ADDR should be child of WS-CUSTOMER-REC
	assertChildOf(t, rels, "CUSTPROG.05.WS-CUST-ADDR", "CUSTPROG.01.WS-CUSTOMER-REC")
	// WS-STREET should be child of WS-CUST-ADDR
	assertChildOf(t, rels, "CUSTPROG.10.WS-STREET", "CUSTPROG.05.WS-CUST-ADDR")
	// WS-CITY should be child of WS-CUST-ADDR
	assertChildOf(t, rels, "CUSTPROG.10.WS-CITY", "CUSTPROG.05.WS-CUST-ADDR")
	// WS-EOF-FLAG should be child of WS-FLAGS
	assertChildOf(t, rels, "CUSTPROG.05.WS-EOF-FLAG", "CUSTPROG.01.WS-FLAGS")
}

func TestExtractDataHierarchy_SkipSpecialLevels(t *testing.T) {
	data := `       DATA DIVISION.
       WORKING-STORAGE SECTION.
       01  WS-STATUS.
           05  WS-CODE           PIC X(1).
           88  WS-OK             VALUE 'Y'.
       77  WS-COUNTER           PIC 9(5).`

	rels := ExtractDataHierarchy(data, "TESTPROG")
	// 88-level and 77-level should be excluded
	for _, r := range rels {
		assert.NotContains(t, r.FromKey, ".88.")
		assert.NotContains(t, r.FromKey, ".77.")
	}
	// WS-CODE should be child of WS-STATUS
	assertChildOf(t, rels, "TESTPROG.05.WS-CODE", "TESTPROG.01.WS-STATUS")
}

func TestExtractDataHierarchy_WithCopybook(t *testing.T) {
	data := `       DATA DIVISION.
       WORKING-STORAGE SECTION.
       01  WS-REC.
      *>> COPY CUSTREC INLINED BEGIN
           05  CUST-ID           PIC X(10).
           05  CUST-NAME         PIC X(30).
      *>> COPY CUSTREC INLINED END
           05  WS-LOCAL          PIC X(5).`

	rels := ExtractDataHierarchy(data, "PROG1")
	// All level-05 items should be children of WS-REC
	assertChildOf(t, rels, "PROG1.05.CUST-ID", "PROG1.01.WS-REC")
	assertChildOf(t, rels, "PROG1.05.CUST-NAME", "PROG1.01.WS-REC")
	assertChildOf(t, rels, "PROG1.05.WS-LOCAL", "PROG1.01.WS-REC")
}

func TestExtractDataHierarchy_EmptyInput(t *testing.T) {
	rels := ExtractDataHierarchy("", "PROG")
	assert.Nil(t, rels)
}

func TestExtractDataHierarchy_DeepNesting(t *testing.T) {
	data := `       01  TOP.
           02  MID.
               03  BOTTOM        PIC X(1).`

	rels := ExtractDataHierarchy(data, "DEEP")
	assertChildOf(t, rels, "DEEP.02.MID", "DEEP.01.TOP")
	assertChildOf(t, rels, "DEEP.03.BOTTOM", "DEEP.02.MID")
}

func TestExtractDataHierarchy_SourceProperty(t *testing.T) {
	data := `       01  REC.
           05  FLD              PIC X(1).`
	rels := ExtractDataHierarchy(data, "P")
	require.Len(t, rels, 1)
	assert.Equal(t, "static_parser", rels[0].Properties["source"])
}

func assertChildOf(t *testing.T, rels []graph.Relationship, childFQN, parentFQN string) {
	t.Helper()
	for _, r := range rels {
		if r.FromKey == childFQN && r.ToKey == parentFQN && r.Type == graph.RelChildOf {
			return
		}
	}
	t.Errorf("expected CHILD_OF from %s to %s, not found in %d relationships", childFQN, parentFQN, len(rels))
}
