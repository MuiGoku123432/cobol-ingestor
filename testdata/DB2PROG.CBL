       IDENTIFICATION DIVISION.
       PROGRAM-ID. DB2PROG.
      *
      * Test program exercising DB2 patterns:
      * INCLUDE SQLCA, DECLARE cursor, SELECT, INSERT, UPDATE, DELETE,
      * and SQLCODE error handling.
      *
       ENVIRONMENT DIVISION.
       DATA DIVISION.
       WORKING-STORAGE SECTION.
           EXEC SQL INCLUDE SQLCA END-EXEC.
       01  WS-CUST-ID          PIC X(10).
       01  WS-CUST-NAME        PIC X(30).
       01  WS-CUST-ADDR        PIC X(50).
       01  WS-CUST-BALANCE     PIC S9(9)V99 COMP-3.
       01  WS-ORDER-ID         PIC X(10).
       01  WS-ORDER-DATE       PIC X(10).
       01  WS-ORDER-AMOUNT     PIC S9(9)V99 COMP-3.
           EXEC SQL DECLARE CUST-CURSOR CURSOR FOR
               SELECT CUST_ID, CUST_NAME, CUST_ADDR
               FROM CUSTOMER
               WHERE CUST_BALANCE > 0
               ORDER BY CUST_NAME
           END-EXEC.
       PROCEDURE DIVISION.
       0000-MAIN.
           PERFORM 1000-SELECT-CUSTOMER
           PERFORM 2000-INSERT-ORDER
           PERFORM 3000-UPDATE-ACCOUNT
           PERFORM 4000-DELETE-TEMP
           PERFORM 9000-CLEANUP
           STOP RUN.
       1000-SELECT-CUSTOMER.
           EXEC SQL
               SELECT CUST_NAME, CUST_ADDR
               INTO :WS-CUST-NAME, :WS-CUST-ADDR
               FROM CUSTOMER
               WHERE CUST_ID = :WS-CUST-ID
           END-EXEC
           IF SQLCODE NOT = 0
               DISPLAY 'SELECT ERROR: ' SQLCODE
               PERFORM 9900-SQL-ERROR
           END-IF.
       2000-INSERT-ORDER.
           EXEC SQL
               INSERT INTO ORDERS
               (ORDER_ID, CUST_ID, ORDER_DATE, ORDER_AMOUNT)
               VALUES
               (:WS-ORDER-ID, :WS-CUST-ID,
                :WS-ORDER-DATE, :WS-ORDER-AMOUNT)
           END-EXEC
           IF SQLCODE NOT = 0
               DISPLAY 'INSERT ERROR: ' SQLCODE
               PERFORM 9900-SQL-ERROR
           END-IF.
       3000-UPDATE-ACCOUNT.
           EXEC SQL
               UPDATE ACCOUNT
               SET LAST_ORDER_DATE = :WS-ORDER-DATE,
                   TOTAL_ORDERS = TOTAL_ORDERS + 1
               WHERE CUST_ID = :WS-CUST-ID
           END-EXEC
           IF SQLCODE NOT = 0
               DISPLAY 'UPDATE ERROR: ' SQLCODE
               PERFORM 9900-SQL-ERROR
           END-IF.
       4000-DELETE-TEMP.
           EXEC SQL
               DELETE FROM TEMP_TABLE
               WHERE PROCESS_DATE < CURRENT DATE - 30 DAYS
           END-EXEC
           IF SQLCODE NOT = 0
               DISPLAY 'DELETE ERROR: ' SQLCODE
               PERFORM 9900-SQL-ERROR
           END-IF.
       9000-CLEANUP.
           EXEC SQL COMMIT END-EXEC.
       9900-SQL-ERROR.
           DISPLAY 'SQLCODE: ' SQLCODE
           DISPLAY 'SQLERRM: ' SQLERRMC
           EXEC SQL ROLLBACK END-EXEC.
