       IDENTIFICATION DIVISION.
       PROGRAM-ID. CICSTEST.
      *
      * Test program exercising CICS online patterns:
      * SEND/RECEIVE MAP, LINK, XCTL, START, WRITEQ/READQ TS/TD,
      * HANDLE CONDITION/AID, and SQLCODE error handling.
      *
       ENVIRONMENT DIVISION.
       DATA DIVISION.
       WORKING-STORAGE SECTION.
       01  WS-RESP-CODE        PIC S9(8) COMP.
       01  WS-TRANSID          PIC X(4) VALUE 'TXN1'.
       01  WS-QUEUE-NAME       PIC X(16) VALUE 'TEMPQ'.
       01  WS-LOG-QUEUE        PIC X(16) VALUE 'LOGQ'.
       01  WS-MAP-DATA         PIC X(80).
       01  WS-TS-DATA          PIC X(200).
       01  WS-SQLCODE          PIC S9(9) COMP.
           COPY SQLCA.
       PROCEDURE DIVISION.
       0000-MAIN.
           PERFORM 1000-INIT
           PERFORM 2000-PROCESS
           PERFORM 3000-CICS-LINK
           PERFORM 4000-CICS-XCTL
           PERFORM 5000-TS-QUEUE
           PERFORM 6000-TD-QUEUE
           PERFORM 7000-START-TRANS
           PERFORM 9000-CLEANUP
           EXEC CICS RETURN END-EXEC
           STOP RUN.
       1000-INIT.
           EXEC CICS HANDLE CONDITION
               ERROR(9900-ERROR-HANDLER)
           END-EXEC
           EXEC CICS HANDLE AID
               PF3(9000-CLEANUP)
               PF12(9000-CLEANUP)
           END-EXEC.
       2000-PROCESS.
           EXEC CICS SEND MAP('CUSTMAP')
               MAPSET('CUSTSET')
               FROM(WS-MAP-DATA)
               ERASE
           END-EXEC
           EXEC CICS RECEIVE MAP('CUSTMAP')
               MAPSET('CUSTSET')
               INTO(WS-MAP-DATA)
           END-EXEC.
       3000-CICS-LINK.
           EXEC CICS LINK PROGRAM('SUBPROG1')
               COMMAREA(WS-MAP-DATA)
               LENGTH(80)
               RESP(WS-RESP-CODE)
           END-EXEC.
       4000-CICS-XCTL.
           EXEC CICS XCTL PROGRAM('NEXTPROG')
               COMMAREA(WS-MAP-DATA)
               LENGTH(80)
           END-EXEC.
       5000-TS-QUEUE.
           EXEC CICS WRITEQ TS QUEUE(WS-QUEUE-NAME)
               FROM(WS-TS-DATA)
               LENGTH(200)
           END-EXEC
           EXEC CICS READQ TS QUEUE(WS-QUEUE-NAME)
               INTO(WS-TS-DATA)
               LENGTH(200)
           END-EXEC.
       6000-TD-QUEUE.
           EXEC CICS WRITEQ TD QUEUE(WS-LOG-QUEUE)
               FROM(WS-TS-DATA)
               LENGTH(200)
           END-EXEC.
       7000-START-TRANS.
           EXEC CICS START TRANSID(WS-TRANSID)
               FROM(WS-MAP-DATA)
               LENGTH(80)
           END-EXEC.
       9000-CLEANUP.
           EXEC CICS SEND TEXT
               FROM('SESSION ENDED')
               ERASE
           END-EXEC.
       9900-ERROR-HANDLER.
           IF WS-SQLCODE NOT = 0
               DISPLAY 'SQL ERROR: ' WS-SQLCODE
           END-IF
           EXEC CICS RETURN END-EXEC.
