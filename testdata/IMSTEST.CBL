       IDENTIFICATION DIVISION.
       PROGRAM-ID. IMSTEST.
      *
      * Test program exercising IMS/DL1 patterns:
      * CBLTDLI calls with GU, GN, ISRT, DLET, REPL function codes.
      * PSB/PCB declarations in LINKAGE SECTION.
      *
       ENVIRONMENT DIVISION.
       DATA DIVISION.
       WORKING-STORAGE SECTION.
       01  WS-FUNCTION-CODE    PIC X(4).
       01  WS-STATUS-CODE      PIC XX.
       01  WS-IO-AREA.
           05  WS-CUST-ID      PIC X(10).
           05  WS-CUST-NAME    PIC X(30).
           05  WS-CUST-ADDR    PIC X(50).
       01  WS-SSA-1.
           05  WS-SEG-NAME     PIC X(8) VALUE 'CUSTSEG '.
           05  WS-SSA-QUAL     PIC X(1) VALUE '('.
           05  WS-SSA-FIELD    PIC X(8) VALUE 'CUSTID  '.
           05  WS-SSA-OP       PIC X(2) VALUE ' ='.
           05  WS-SSA-VALUE    PIC X(10).
           05  WS-SSA-CLOSE    PIC X(1) VALUE ')'.
       LINKAGE SECTION.
       01  CUSTOMER-PCB.
           05  PCB-DBD-NAME    PIC X(8).
           05  PCB-SEG-LEVEL   PIC XX.
           05  PCB-STATUS      PIC XX.
           05  PCB-PROC-OPT    PIC X(4).
           05  PCB-RESERVED    PIC S9(5) COMP.
           05  PCB-SEG-NAME    PIC X(8).
           05  PCB-KEY-LENGTH  PIC S9(5) COMP.
           05  PCB-SENS-SEGS   PIC S9(5) COMP.
           05  PCB-KEY-AREA    PIC X(22).
       PROCEDURE DIVISION USING CUSTOMER-PCB.
       0000-MAIN.
           PERFORM 1000-GET-UNIQUE
           PERFORM 2000-GET-NEXT
           PERFORM 3000-INSERT
           PERFORM 4000-DELETE
           PERFORM 5000-REPLACE
           GOBACK.
       1000-GET-UNIQUE.
           MOVE 'GU  ' TO WS-FUNCTION-CODE
           MOVE 'CUST000001' TO WS-SSA-VALUE
           CALL 'CBLTDLI' USING
               WS-FUNCTION-CODE
               CUSTOMER-PCB
               WS-IO-AREA
               WS-SSA-1.
           MOVE PCB-STATUS TO WS-STATUS-CODE
           IF WS-STATUS-CODE NOT = SPACES
               DISPLAY 'GU FAILED: ' WS-STATUS-CODE
           END-IF.
       2000-GET-NEXT.
           MOVE 'GN  ' TO WS-FUNCTION-CODE
           CALL 'CBLTDLI' USING
               WS-FUNCTION-CODE
               CUSTOMER-PCB
               WS-IO-AREA.
           MOVE PCB-STATUS TO WS-STATUS-CODE.
       3000-INSERT.
           MOVE 'ISRT' TO WS-FUNCTION-CODE
           MOVE 'CUST000099' TO WS-CUST-ID
           MOVE 'NEW CUSTOMER' TO WS-CUST-NAME
           CALL 'CBLTDLI' USING
               WS-FUNCTION-CODE
               CUSTOMER-PCB
               WS-IO-AREA.
       4000-DELETE.
           MOVE 'DLET' TO WS-FUNCTION-CODE
           CALL 'CBLTDLI' USING
               WS-FUNCTION-CODE
               CUSTOMER-PCB.
       5000-REPLACE.
           MOVE 'REPL' TO WS-FUNCTION-CODE
           MOVE 'UPDATED NAME' TO WS-CUST-NAME
           CALL 'CBLTDLI' USING
               WS-FUNCTION-CODE
               CUSTOMER-PCB
               WS-IO-AREA.
