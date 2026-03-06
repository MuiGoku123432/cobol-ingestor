      * Customer Record Copybook
       01 WS-CUSTOMER-RECORD.
           05 WS-CUST-ID       PIC X(10).
           05 WS-CUST-NAME     PIC X(30).
           05 WS-CUST-ADDR.
               10 WS-STREET    PIC X(30).
               10 WS-CITY      PIC X(20).
               10 WS-STATE     PIC XX.
               10 WS-ZIP       PIC X(10).
           05 WS-CUST-STATUS   PIC X.
               88 ACTIVE       VALUE 'A'.
               88 INACTIVE     VALUE 'I'.
           05 WS-CUST-BALANCE  PIC 9(7)V99.
           05 WS-LAST-UPDATE   PIC X(8).
