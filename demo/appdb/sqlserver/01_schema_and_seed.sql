-- ============================================================================
-- DEMO "ERP" database for SQL Server (equivalent of the PostgreSQL demo).
-- Use this when presenting against a SQL Server / IIS / WebForms environment.
--   sqlcmd -S localhost -U sa -P <pwd> -i 01_schema_and_seed.sql
-- ============================================================================
IF DB_ID('ErpDemo') IS NULL CREATE DATABASE ErpDemo;
GO
USE ErpDemo;
GO
IF OBJECT_ID('dbo.Payments') IS NOT NULL DROP TABLE dbo.Payments;
IF OBJECT_ID('dbo.GLEntries') IS NOT NULL DROP TABLE dbo.GLEntries;
IF OBJECT_ID('dbo.Invoices') IS NOT NULL DROP TABLE dbo.Invoices;
IF OBJECT_ID('dbo.Orders') IS NOT NULL DROP TABLE dbo.Orders;
IF OBJECT_ID('dbo.Inventory') IS NOT NULL DROP TABLE dbo.Inventory;
IF OBJECT_ID('dbo.Customers') IS NOT NULL DROP TABLE dbo.Customers;
IF OBJECT_ID('dbo.AppErrorLog') IS NOT NULL DROP TABLE dbo.AppErrorLog;
GO
CREATE TABLE dbo.Customers (CustomerId INT IDENTITY PRIMARY KEY, Name NVARCHAR(100) NOT NULL, City NVARCHAR(50) NOT NULL);
CREATE TABLE dbo.Inventory (Sku VARCHAR(20) NOT NULL, Warehouse VARCHAR(20) NOT NULL, QtyOnHand INT NOT NULL, PRIMARY KEY (Sku, Warehouse));
CREATE TABLE dbo.Orders (OrderId INT IDENTITY PRIMARY KEY, CustomerId INT NOT NULL REFERENCES dbo.Customers, Status VARCHAR(20) NOT NULL,
                         TotalAmount DECIMAL(14,2) NOT NULL, CreatedAt DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(), UpdatedAt DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME());
CREATE TABLE dbo.Invoices (InvoiceNo VARCHAR(20) PRIMARY KEY, OrderId INT REFERENCES dbo.Orders, Amount DECIMAL(14,2) NOT NULL, Posted BIT NOT NULL DEFAULT 0, PostedAt DATETIME2 NULL);
CREATE TABLE dbo.GLEntries (EntryId INT IDENTITY PRIMARY KEY, InvoiceNo VARCHAR(20) REFERENCES dbo.Invoices, Account VARCHAR(40) NOT NULL, Debit DECIMAL(14,2) NOT NULL DEFAULT 0, Credit DECIMAL(14,2) NOT NULL DEFAULT 0);
CREATE TABLE dbo.Payments (PaymentId INT IDENTITY PRIMARY KEY, InvoiceNo VARCHAR(20) REFERENCES dbo.Invoices, Amount DECIMAL(14,2) NOT NULL, Reference VARCHAR(40) NOT NULL, PaidAt DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME());
CREATE TABLE dbo.AppErrorLog (LogId BIGINT IDENTITY PRIMARY KEY, AppName NVARCHAR(50) NOT NULL, Level VARCHAR(10) NOT NULL, Message NVARCHAR(2000) NOT NULL,
                              StackTrace NVARCHAR(MAX) NULL, UserName NVARCHAR(50) NULL, LoggedAt DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME());
GO
INSERT dbo.Customers (Name, City) VALUES (N'Al-Noor Traders',N'Karachi'),(N'Punjab Textiles Ltd',N'Lahore'),(N'Margalla Pharma',N'Islamabad'),(N'Indus Steel Works',N'Hyderabad');
INSERT dbo.Inventory VALUES ('SKU-1001','KHI-01',420),('SKU-1004','KHI-01',-12),('SKU-1005','LHE-01',-40),('SKU-1006','SKT-01',310);
INSERT dbo.Orders (CustomerId, Status, TotalAmount, CreatedAt, UpdatedAt) VALUES
 (1,'SHIPPED',150000,DATEADD(DAY,-2,SYSUTCDATETIME()),DATEADD(DAY,-2,SYSUTCDATETIME())),
 (2,'PROCESSING',845000,DATEADD(HOUR,-5,SYSUTCDATETIME()),DATEADD(HOUR,-5,SYSUTCDATETIME())),
 (4,'PROCESSING',1325000,DATEADD(HOUR,-3,SYSUTCDATETIME()),DATEADD(HOUR,-3,SYSUTCDATETIME())),
 (3,'PROCESSING',96000,DATEADD(MINUTE,-20,SYSUTCDATETIME()),DATEADD(MINUTE,-20,SYSUTCDATETIME()));
INSERT dbo.Invoices VALUES ('INV-00001',1,150000,1,SYSUTCDATETIME()),('INV-90001',2,275000,1,SYSUTCDATETIME());
INSERT dbo.GLEntries (InvoiceNo, Account, Debit, Credit) VALUES ('INV-00001','1200-Receivables',150000,0),('INV-00001','4000-Sales',0,150000);
INSERT dbo.Payments (InvoiceNo, Amount, Reference) VALUES ('INV-00001',150000,'BANK-A1'),('INV-00001',150000,'BANK-DUP-7781');
INSERT dbo.AppErrorLog (AppName, Level, Message) VALUES (N'OrderPortal','INFO',N'Application started');
GO
-- Read-only monitoring login
IF SUSER_ID('sentinel_ro') IS NULL CREATE LOGIN sentinel_ro WITH PASSWORD = 'Sentinel_ro#2026';
IF USER_ID('sentinel_ro') IS NULL CREATE USER sentinel_ro FOR LOGIN sentinel_ro;
ALTER ROLE db_datareader ADD MEMBER sentinel_ro;
GO
/* Example rules for SQL Server (paste into "Exception Rules"):
   SELECT i.InvoiceNo, i.Amount, i.PostedAt FROM dbo.Invoices i
    WHERE i.Posted = 1 AND NOT EXISTS (SELECT 1 FROM dbo.GLEntries g WHERE g.InvoiceNo = i.InvoiceNo);
   SELECT Sku, Warehouse, QtyOnHand FROM dbo.Inventory WHERE QtyOnHand < 0;
   SELECT OrderId, CustomerId, TotalAmount, CreatedAt FROM dbo.Orders
    WHERE Status = 'PROCESSING' AND CreatedAt < DATEADD(HOUR, -2, SYSUTCDATETIME());
   Log watcher: table dbo.AppErrorLog, id column LogId, filter  Level IN ('ERROR','FATAL')
   DSN: sqlserver://sentinel_ro:Sentinel_ro%232026@localhost:1433?database=ErpDemo
*/
