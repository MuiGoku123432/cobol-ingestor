       IDENTIFICATION DIVISION.
       PROGRAM-ID. ADABAS.
      *
      * Test program exercising ADABAS database patterns:
      * CALL 'ADABAS' with control block for various commands.
      *
       ENVIRONMENT DIVISION.
       DATA DIVISION.
       WORKING-STORAGE SECTION.
       01  ADABAS-CB.
           05  CB-COMMAND-CODE       PIC X(2).
           05  CB-COMMAND-ID         PIC X(4).
           05  CB-FILE-NUMBER        PIC 9(5).
           05  CB-RESPONSE-CODE      PIC 9(4).
           05  CB-ISN                PIC 9(10).
           05  CB-ISN-LOWER-LIMIT    PIC 9(10).
           05  CB-ISN-QUANTITY       PIC 9(10).
           05  CB-FORMAT-BUFFER-LEN  PIC 9(5).
           05  CB-RECORD-BUFFER-LEN  PIC 9(5).
           05  CB-SEARCH-BUFFER-LEN  PIC 9(5).
           05  CB-VALUE-BUFFER-LEN   PIC 9(5).
       01  FORMAT-BUFFER             PIC X(200).
       01  RECORD-BUFFER             PIC X(2000).
       01  SEARCH-BUFFER             PIC X(200).
       01  VALUE-BUFFER              PIC X(200).
       01  WS-EMPLOYEE-REC.
           05  WS-EMP-ID            PIC X(8).
           05  WS-EMP-NAME          PIC X(30).
           05  WS-EMP-DEPT          PIC X(10).
       PROCEDURE DIVISION.
       0000-MAIN.
           PERFORM 1000-READ-RECORD
           PERFORM 2000-FIND-RECORDS
           PERFORM 3000-STORE-RECORD
           PERFORM 4000-UPDATE-RECORD
           PERFORM 5000-DELETE-RECORD
           GOBACK.
       1000-READ-RECORD.
           MOVE 'L3' TO CB-COMMAND-CODE
           MOVE 00020 TO CB-FILE-NUMBER
           MOVE 'AA,AB,AC.' TO FORMAT-BUFFER
           CALL 'ADABAS' USING
               ADABAS-CB
               FORMAT-BUFFER
               RECORD-BUFFER
               SEARCH-BUFFER
               VALUE-BUFFER.
           IF CB-RESPONSE-CODE NOT = 0
               DISPLAY 'ADABAS L3 FAILED: ' CB-RESPONSE-CODE
           END-IF.
       2000-FIND-RECORDS.
           MOVE 'S1' TO CB-COMMAND-CODE
           MOVE 00020 TO CB-FILE-NUMBER
           MOVE 'AA.' TO SEARCH-BUFFER
           MOVE 'EMP001' TO VALUE-BUFFER
           CALL 'ADABAS' USING
               ADABAS-CB
               FORMAT-BUFFER
               RECORD-BUFFER
               SEARCH-BUFFER
               VALUE-BUFFER.
           IF CB-RESPONSE-CODE NOT = 0
               DISPLAY 'ADABAS S1 FAILED: ' CB-RESPONSE-CODE
           END-IF.
       3000-STORE-RECORD.
           MOVE 'N1' TO CB-COMMAND-CODE
           MOVE 00020 TO CB-FILE-NUMBER
           MOVE 'AA,AB,AC.' TO FORMAT-BUFFER
           MOVE WS-EMPLOYEE-REC TO RECORD-BUFFER
           CALL 'ADABAS' USING
               ADABAS-CB
               FORMAT-BUFFER
               RECORD-BUFFER
               SEARCH-BUFFER
               VALUE-BUFFER.
           IF CB-RESPONSE-CODE NOT = 0
               DISPLAY 'ADABAS N1 FAILED: ' CB-RESPONSE-CODE
           END-IF.
       4000-UPDATE-RECORD.
           MOVE 'A1' TO CB-COMMAND-CODE
           MOVE 00020 TO CB-FILE-NUMBER
           MOVE 'AB.' TO FORMAT-BUFFER
           MOVE 'UPDATED NAME' TO RECORD-BUFFER
           CALL 'ADABAS' USING
               ADABAS-CB
               FORMAT-BUFFER
               RECORD-BUFFER
               SEARCH-BUFFER
               VALUE-BUFFER.
           IF CB-RESPONSE-CODE NOT = 0
               DISPLAY 'ADABAS A1 FAILED: ' CB-RESPONSE-CODE
           END-IF.
       5000-DELETE-RECORD.
           MOVE 'E1' TO CB-COMMAND-CODE
           MOVE 00020 TO CB-FILE-NUMBER
           CALL 'ADABAS' USING
               ADABAS-CB
               FORMAT-BUFFER
               RECORD-BUFFER
               SEARCH-BUFFER
               VALUE-BUFFER.
           IF CB-RESPONSE-CODE NOT = 0
               DISPLAY 'ADABAS E1 FAILED: ' CB-RESPONSE-CODE
           END-IF.
