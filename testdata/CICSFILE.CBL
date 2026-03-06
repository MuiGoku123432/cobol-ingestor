       IDENTIFICATION DIVISION.
       PROGRAM-ID. CICSFILE.
      *
      * Test program exercising CICS file I/O and ENQ/DEQ:
      * READ, WRITE, REWRITE, DELETE, STARTBR, READNEXT,
      * READPREV, ENDBR, ENQ, DEQ.
      *
       ENVIRONMENT DIVISION.
       DATA DIVISION.
       WORKING-STORAGE SECTION.
       01  WS-CUST-REC.
           05  WS-CUST-ID           PIC X(10).
           05  WS-CUST-NAME         PIC X(30).
           05  WS-CUST-ADDR         PIC X(50).
           05  WS-CUST-BAL          PIC S9(9)V99 COMP-3.
       01  WS-RESP                   PIC S9(8) COMP.
       01  WS-RIDFLD                 PIC X(10).
       01  WS-LOCK-RESOURCE         PIC X(20).
       01  WS-LOCK-LENGTH           PIC S9(4) COMP VALUE 20.
       PROCEDURE DIVISION.
       0000-MAIN.
           PERFORM 1000-READ-RECORD
           PERFORM 2000-WRITE-RECORD
           PERFORM 3000-REWRITE-RECORD
           PERFORM 4000-DELETE-RECORD
           PERFORM 5000-BROWSE-FORWARD
           PERFORM 6000-BROWSE-BACKWARD
           PERFORM 7000-ENQ-DEQ
           EXEC CICS RETURN END-EXEC.
       1000-READ-RECORD.
           MOVE 'CUST000001' TO WS-RIDFLD
           EXEC CICS READ
               FILE('CUSTFILE')
               INTO(WS-CUST-REC)
               RIDFLD(WS-RIDFLD)
               RESP(WS-RESP)
           END-EXEC.
           IF WS-RESP NOT = DFHRESP(NORMAL)
               DISPLAY 'READ FAILED: ' WS-RESP
           END-IF.
       2000-WRITE-RECORD.
           MOVE 'CUST000099' TO WS-CUST-ID
           MOVE 'NEW CUSTOMER' TO WS-CUST-NAME
           EXEC CICS WRITE
               FILE('CUSTFILE')
               FROM(WS-CUST-REC)
               RIDFLD(WS-CUST-ID)
               RESP(WS-RESP)
           END-EXEC.
           IF WS-RESP NOT = DFHRESP(NORMAL)
               DISPLAY 'WRITE FAILED: ' WS-RESP
           END-IF.
       3000-REWRITE-RECORD.
           MOVE 'UPDATED ADDR' TO WS-CUST-ADDR
           EXEC CICS REWRITE
               FILE('CUSTFILE')
               FROM(WS-CUST-REC)
               RESP(WS-RESP)
           END-EXEC.
           IF WS-RESP NOT = DFHRESP(NORMAL)
               DISPLAY 'REWRITE FAILED: ' WS-RESP
           END-IF.
       4000-DELETE-RECORD.
           MOVE 'CUST000099' TO WS-RIDFLD
           EXEC CICS DELETE
               FILE('CUSTFILE')
               RIDFLD(WS-RIDFLD)
               RESP(WS-RESP)
           END-EXEC.
           IF WS-RESP NOT = DFHRESP(NORMAL)
               DISPLAY 'DELETE FAILED: ' WS-RESP
           END-IF.
       5000-BROWSE-FORWARD.
           MOVE 'CUST000001' TO WS-RIDFLD
           EXEC CICS STARTBR
               FILE('CUSTFILE')
               RIDFLD(WS-RIDFLD)
               RESP(WS-RESP)
           END-EXEC
           PERFORM UNTIL WS-RESP NOT = DFHRESP(NORMAL)
               EXEC CICS READNEXT
                   FILE('CUSTFILE')
                   INTO(WS-CUST-REC)
                   RIDFLD(WS-RIDFLD)
                   RESP(WS-RESP)
               END-EXEC
           END-PERFORM
           EXEC CICS ENDBR
               FILE('CUSTFILE')
               RESP(WS-RESP)
           END-EXEC.
       6000-BROWSE-BACKWARD.
           MOVE 'CUST999999' TO WS-RIDFLD
           EXEC CICS STARTBR
               FILE('CUSTFILE')
               RIDFLD(WS-RIDFLD)
               RESP(WS-RESP)
           END-EXEC
           PERFORM UNTIL WS-RESP NOT = DFHRESP(NORMAL)
               EXEC CICS READPREV
                   FILE('CUSTFILE')
                   INTO(WS-CUST-REC)
                   RIDFLD(WS-RIDFLD)
                   RESP(WS-RESP)
               END-EXEC
           END-PERFORM
           EXEC CICS ENDBR
               FILE('CUSTFILE')
               RESP(WS-RESP)
           END-EXEC.
       7000-ENQ-DEQ.
           MOVE 'CUST-UPDATE-LOCK' TO WS-LOCK-RESOURCE
           EXEC CICS ENQ
               RESOURCE(WS-LOCK-RESOURCE)
               LENGTH(WS-LOCK-LENGTH)
               RESP(WS-RESP)
           END-EXEC
           IF WS-RESP = DFHRESP(NORMAL)
               PERFORM 1000-READ-RECORD
               PERFORM 3000-REWRITE-RECORD
               EXEC CICS DEQ
                   RESOURCE(WS-LOCK-RESOURCE)
                   LENGTH(WS-LOCK-LENGTH)
                   RESP(WS-RESP)
               END-EXEC
           END-IF.
