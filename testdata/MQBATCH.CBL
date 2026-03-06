       IDENTIFICATION DIVISION.
       PROGRAM-ID. MQBATCH.
      *
      * Test program exercising MQ batch patterns:
      * MQCONN, MQOPEN, MQPUT, MQGET, MQCLOSE, MQDISC.
      *
       ENVIRONMENT DIVISION.
       DATA DIVISION.
       WORKING-STORAGE SECTION.
       01  WS-HCONN            PIC S9(9) COMP.
       01  WS-HOBJ             PIC S9(9) COMP.
       01  WS-COMPCODE         PIC S9(9) COMP.
       01  WS-REASON           PIC S9(9) COMP.
       01  WS-QMGR-NAME       PIC X(48) VALUE 'QMGR01'.
       01  WS-QUEUE-NAME       PIC X(48) VALUE 'ORDER.REQUEST.Q'.
       01  WS-MSG-BUFFER       PIC X(4096).
       01  WS-MSG-LENGTH       PIC S9(9) COMP VALUE 4096.
       01  WS-OBJ-DESC.
           05  WS-OD-TYPE      PIC S9(9) COMP VALUE 1.
           05  WS-OD-NAME      PIC X(48).
       01  WS-MSG-DESC.
           05  WS-MD-FORMAT    PIC X(8) VALUE 'MQSTR'.
       01  WS-PUT-OPTIONS.
           05  WS-PMO-OPTIONS  PIC S9(9) COMP.
       01  WS-GET-OPTIONS.
           05  WS-GMO-OPTIONS  PIC S9(9) COMP.
       PROCEDURE DIVISION.
       0000-MAIN.
           PERFORM 1000-CONNECT
           PERFORM 2000-OPEN-QUEUE
           PERFORM 3000-PUT-MESSAGE
           PERFORM 4000-GET-MESSAGE
           PERFORM 5000-CLOSE-QUEUE
           PERFORM 6000-DISCONNECT
           STOP RUN.
       1000-CONNECT.
           CALL 'MQCONN' USING
               WS-QMGR-NAME
               WS-HCONN
               WS-COMPCODE
               WS-REASON.
           IF WS-COMPCODE NOT = 0
               DISPLAY 'MQCONN FAILED: ' WS-REASON
               STOP RUN
           END-IF.
       2000-OPEN-QUEUE.
           MOVE WS-QUEUE-NAME TO WS-OD-NAME
           CALL 'MQOPEN' USING
               WS-HCONN
               WS-OBJ-DESC
               WS-COMPCODE
               WS-REASON
               WS-HOBJ.
           IF WS-COMPCODE NOT = 0
               DISPLAY 'MQOPEN FAILED: ' WS-REASON
           END-IF.
       3000-PUT-MESSAGE.
           MOVE 'ORDER-12345' TO WS-MSG-BUFFER
           CALL 'MQPUT' USING
               WS-HCONN
               WS-HOBJ
               WS-MSG-DESC
               WS-PUT-OPTIONS
               WS-MSG-LENGTH
               WS-MSG-BUFFER
               WS-COMPCODE
               WS-REASON.
       4000-GET-MESSAGE.
           CALL 'MQGET' USING
               WS-HCONN
               WS-HOBJ
               WS-MSG-DESC
               WS-GET-OPTIONS
               WS-MSG-LENGTH
               WS-MSG-BUFFER
               WS-COMPCODE
               WS-REASON.
       5000-CLOSE-QUEUE.
           CALL 'MQCLOSE' USING
               WS-HCONN
               WS-HOBJ
               WS-COMPCODE
               WS-REASON.
       6000-DISCONNECT.
           CALL 'MQDISC' USING
               WS-HCONN
               WS-COMPCODE
               WS-REASON.
