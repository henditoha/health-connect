// Database connection templates, default ports, and DSN format builders
export const DB_TEMPLATES = {
  postgres: {
    code: 'postgres',
    name: 'PostgreSQL',
    defaultPort: '5432',
    defaultUser: 'postgres',
    defaultDatabase: 'hospital_db',
    defaultOptions: 'sslmode=disable',
    sample: 'postgres://postgres:password@localhost:5432/hospital_db?sslmode=disable',
    placeholder: 'postgres://user:password@host:5432/dbname?sslmode=disable',
    syntax: 'postgres://[user]:[password]@[host]:[port]/[database]?[options]',
    description: 'PostgreSQL 12+ (SIMRS, PACS, Satu Sehat, OpenMRS)',
    presets: [
      { label: 'Localhost (No SSL)', dsn: 'postgres://postgres:password@localhost:5432/hospital_db?sslmode=disable' },
      { label: 'LAN IP Server', dsn: 'postgres://postgres:simrs123@192.168.1.100:5432/simrs?sslmode=disable' },
      { label: 'Cloud / Supabase (SSL)', dsn: 'postgres://postgres:password@aws-0-ap-southeast-1.pooler.supabase.com:6543/postgres?sslmode=require' }
    ],
    build: ({ host, port, user, password, database, options }) => {
      const h = host || 'localhost';
      const p = port || '5432';
      const u = user || 'postgres';
      const pw = password ? `:${password}` : '';
      const db = database || 'hospital_db';
      const opt = options || 'sslmode=disable';
      return `postgres://${u}${pw}@${h}:${p}/${db}?${opt}`;
    }
  },
  mysql: {
    code: 'mysql',
    name: 'MySQL',
    defaultPort: '3306',
    defaultUser: 'root',
    defaultDatabase: 'simrs_db',
    defaultOptions: 'parseTime=true',
    sample: 'root:password@tcp(localhost:3306)/simrs_db?parseTime=true',
    placeholder: 'user:password@tcp(host:3306)/dbname?parseTime=true',
    syntax: '[user]:[password]@tcp([host]:[port])/[database]?[options]',
    description: 'MySQL 5.7 / 8.0+ (Khanza, Medin, SIMRS Kemenkes)',
    presets: [
      { label: 'Localhost Standar', dsn: 'root:password@tcp(localhost:3306)/simrs_db?parseTime=true' },
      { label: 'LAN IP Server', dsn: 'simrs_user:SimrsSecret123@tcp(192.168.1.50:3306)/simrs?parseTime=true&loc=Local' },
      { label: 'Docker Container', dsn: 'root:rootpassword@tcp(127.0.0.1:3306)/khanza?parseTime=true' }
    ],
    build: ({ host, port, user, password, database, options }) => {
      const h = host || 'localhost';
      const p = port || '3306';
      const u = user || 'root';
      const pw = password ? `:${password}` : '';
      const db = database || 'simrs_db';
      const opt = options || 'parseTime=true';
      return `${u}${pw}@tcp(${h}:${p})/${db}?${opt}`;
    }
  },
  mariadb: {
    code: 'mariadb',
    name: 'MariaDB',
    defaultPort: '3306',
    defaultUser: 'root',
    defaultDatabase: 'simrs_db',
    defaultOptions: 'parseTime=true',
    sample: 'root:password@tcp(localhost:3306)/simrs_db?parseTime=true',
    placeholder: 'user:password@tcp(host:3306)/dbname?parseTime=true',
    syntax: '[user]:[password]@tcp([host]:[port])/[database]?[options]',
    description: 'MariaDB 10.x+ (100% kompatibel protokol MySQL)',
    presets: [
      { label: 'Localhost Standar', dsn: 'root:password@tcp(localhost:3306)/simrs_db?parseTime=true' },
      { label: 'LAN IP Server', dsn: 'mariadb_user:Secret123@tcp(192.168.1.50:3306)/simrs?parseTime=true' }
    ],
    build: ({ host, port, user, password, database, options }) => {
      const h = host || 'localhost';
      const p = port || '3306';
      const u = user || 'root';
      const pw = password ? `:${password}` : '';
      const db = database || 'simrs_db';
      const opt = options || 'parseTime=true';
      return `${u}${pw}@tcp(${h}:${p})/${db}?${opt}`;
    }
  },
  sqlserver: {
    code: 'sqlserver',
    name: 'Microsoft SQL Server',
    defaultPort: '1433',
    defaultUser: 'sa',
    defaultDatabase: 'SIMRS',
    defaultOptions: 'database=SIMRS',
    sample: 'sqlserver://sa:StrongPassword123@localhost:1433?database=SIMRS',
    placeholder: 'sqlserver://user:password@host:1433?database=dbname',
    syntax: 'sqlserver://[user]:[password]@[host]:[port]?database=[database]',
    description: 'Microsoft SQL Server 2012 / 2016 / 2019 / 2022',
    presets: [
      { label: 'Default SQL Server', dsn: 'sqlserver://sa:StrongPassword123@localhost:1433?database=SIMRS' },
      { label: 'Named Instance (SQLEXPRESS)', dsn: 'sqlserver://sa:Password123@192.168.1.20:1433?instance=SQLEXPRESS&database=HIS' },
      { label: 'Windows (Encrypt Disable)', dsn: 'sqlserver://sa:Password123@localhost:1433?database=SIMRS&encrypt=disable' }
    ],
    build: ({ host, port, user, password, database, options }) => {
      const h = host || 'localhost';
      const p = port || '1433';
      const u = user || 'sa';
      const pw = password ? `:${password}` : '';
      const db = database ? `database=${database}` : 'database=SIMRS';
      let opt = options ? `&${options}` : '';
      return `sqlserver://${u}${pw}@${h}:${p}?${db}${opt}`;
    }
  },
  sqlite: {
    code: 'sqlite',
    name: 'SQLite',
    defaultPort: '',
    defaultUser: '',
    defaultDatabase: 'middleware.db',
    defaultOptions: '',
    sample: 'middleware.db',
    placeholder: 'middleware.db or /path/to/simrs.db',
    syntax: '[nama_file.db] atau path lengkap [/data/simrs.db]',
    description: 'Embedded SQLite Database (Internal Health-Connect / File)',
    presets: [
      { label: 'Internal Health-Connect DB', dsn: 'middleware.db' },
      { label: 'SIMRS SQLite Lokal', dsn: 'simrs.db' },
      { label: 'Absolute Path', dsn: '/Volumes/Data 1/health-connect/simrs.db' }
    ],
    build: ({ database }) => {
      return database || 'middleware.db';
    }
  },
  odbc: {
    code: 'odbc',
    name: 'InterSystems IRIS / Caché (ODBC)',
    defaultPort: '1972',
    defaultUser: '_SYSTEM',
    defaultDatabase: 'USER',
    defaultOptions: '',
    sample: 'DSN=SIMRS_IRIS;UID=_SYSTEM;PWD=SYS;',
    placeholder: 'DSN=System_DSN_Name;UID=user;PWD=pass;',
    syntax: 'DSN=[Nama_DSN];UID=[user];PWD=[pass]; atau Driver={InterSystems IRIS ODBC35};...',
    description: 'InterSystems IRIS & Caché (Standar TrakCare, TrakCare Lab, HIS)',
    presets: [
      { label: 'System DSN (TrakCare IRIS)', dsn: 'DSN=SIMRS_IRIS;UID=_SYSTEM;PWD=SYS;' },
      { label: 'TrakCare Production DSN', dsn: 'DSN=TRAKCARE_PROD;UID=trakcare;PWD=trakcare123;' },
      { label: 'Direct IRIS Driver (No DSN)', dsn: 'Driver={InterSystems IRIS ODBC35};Server=localhost;Port=1972;Database=USER;UID=_SYSTEM;PWD=SYS;' }
    ],
    build: ({ host, port, user, password, database }) => {
      if (host && host.includes('=')) {
        return host; // DSN string provided directly
      }
      if (host && host !== 'localhost' && !host.startsWith('DSN=')) {
        return `Driver={InterSystems IRIS ODBC35};Server=${host};Port=${port || '1972'};Database=${database || 'USER'};UID=${user || '_SYSTEM'};PWD=${password || 'SYS'};`;
      }
      return `DSN=${database || 'SIMRS_IRIS'};UID=${user || '_SYSTEM'};PWD=${password || 'SYS'};`;
    }
  },
  oracle: {
    code: 'oracle',
    name: 'Oracle Database',
    defaultPort: '1521',
    defaultUser: 'system',
    defaultDatabase: 'ORCL',
    defaultOptions: '',
    sample: 'oracle://system:manager@localhost:1521/ORCL',
    placeholder: 'oracle://user:password@host:1521/SERVICE_NAME',
    syntax: 'oracle://[user]:[password]@[host]:1521/[service_name] atau DSN=OracleODBC;UID=...;',
    description: 'Oracle Database 11g / 12c / 19c / 21c',
    presets: [
      { label: 'Oracle Service Name', dsn: 'oracle://system:manager@localhost:1521/ORCL' },
      { label: 'Oracle SID', dsn: 'oracle://simrs:password123@192.168.1.30:1521/XE' },
      { label: 'Oracle via ODBC', dsn: 'DSN=ORACLE_SIMRS;UID=simrs;PWD=secret;' }
    ],
    build: ({ host, port, user, password, database }) => {
      const h = host || 'localhost';
      const p = port || '1521';
      const u = user || 'system';
      const pw = password ? `:${password}` : '';
      const db = database || 'ORCL';
      return `oracle://${u}${pw}@${h}:${p}/${db}`;
    }
  },
  hdb: {
    code: 'hdb',
    name: 'SAP HANA',
    defaultPort: '39015',
    defaultUser: 'SYSTEM',
    defaultDatabase: '',
    defaultOptions: '',
    sample: 'hdb://SYSTEM:Password123@localhost:39015',
    placeholder: 'hdb://user:password@host:39015',
    syntax: 'hdb://[user]:[password]@[host]:[port]',
    description: 'SAP HANA Database (ERP Rumah Sakit / SAP Healthcare)',
    presets: [
      { label: 'SAP HANA Default', dsn: 'hdb://SYSTEM:Password123@localhost:39015' },
      { label: 'SAP HANA Multi-Tenant', dsn: 'hdb://SYSTEM:Password123@192.168.1.40:39015?databaseName=HDB' }
    ],
    build: ({ host, port, user, password, options }) => {
      const h = host || 'localhost';
      const p = port || '39015';
      const u = user || 'SYSTEM';
      const pw = password ? `:${password}` : '';
      const opt = options ? `?${options}` : '';
      return `hdb://${u}${pw}@${h}:${p}${opt}`;
    }
  }
};
