# OneTerm Deployment Scripts

This directory contains scripts to help you deploy and manage OneTerm with custom database passwords.

## Quick Start

### For New Installations

If you're installing OneTerm for the first time:

```bash
# Clone the repository and navigate to deploy directory
cd oneterm/deploy

# Run the setup script to configure passwords
./setup.sh

# Start OneTerm services
docker compose up -d

# Check service status
docker compose ps
```

### For Existing Installations

If you already have OneTerm running and want to change passwords:

```bash
# Navigate to your OneTerm deploy directory
cd oneterm/deploy

# Run the migration script
./migrate-passwords.sh

# The script will guide you through the password change process
```

## Script Details

### setup.sh

**Purpose**: Initial configuration for new OneTerm deployments  
**When to use**: Before first `docker compose up` command  
**What it does**:
- Configures database passwords in all configuration files
- Creates backup of original files
- Generates random passwords or accepts custom ones
- Updates docker-compose.yaml, config.yaml, create-users.sql, and .env

**Usage**:
```bash
./setup.sh
```

**Options**:
- Option 1: Enter custom passwords manually
- Option 2: Generate random passwords automatically (recommended)

### migrate-passwords.sh

**Purpose**: Password migration for existing OneTerm installations  
**When to use**: When OneTerm is already running and you want to change passwords  
**What it does**:
- Creates database backup before changes
- Verifies current passwords
- Updates database user passwords
- Updates all configuration files
- Restarts services with new configuration
- Performs health checks

**Usage**:
```bash
./migrate-passwords.sh
```

**Safety features**:
- Creates automatic backups
- Verifies passwords before proceeding
- Graceful service restart
- Rollback instructions if issues occur

## File Structure

After running setup.sh, your directory will contain:

```
deploy/
├── docker-compose.yaml       # Updated with new MySQL root password
├── config.yaml              # Updated with OneTerm DB password
├── create-users.sql          # Updated with ACL/OneTerm passwords
├── .env                      # Updated with ACL configuration
├── .passwords               # Password reference file (keep secure!)
├── setup.sh                 # Setup script for new installations
├── migrate-passwords.sh     # Migration script for existing systems
└── *.backup.*              # Backup files (created during migration)
```

## Security Considerations

1. **Password Storage**: The `.passwords` file contains sensitive information. Keep it secure and consider deleting after noting passwords elsewhere.

2. **Backup Files**: Migration creates backup files with timestamps. Review and clean up old backups periodically.

3. **Default Passwords**: Never use default passwords (123456) in production environments.

4. **Network Security**: Ensure proper firewall configuration and network isolation.

## Troubleshooting

### Services Won't Start After Password Change

1. Check service logs:
   ```bash
   docker compose logs oneterm-api
   docker compose logs oneterm-acl-api
   docker compose logs oneterm-mysql
   ```

2. Verify database connections:
   ```bash
   docker exec oneterm-mysql mysql -u root -p[NEW_PASSWORD] -e "SELECT 1;"
   ```

3. If migration failed, restore from backup:
   ```bash
   # Stop services
   docker compose down
   
   # Restore configuration files
   cp docker-compose.yaml.backup.TIMESTAMP docker-compose.yaml
   cp config.yaml.backup.TIMESTAMP config.yaml
   cp .env.backup.TIMESTAMP .env
   
   # Restore database if needed
   docker compose up -d mysql
   docker exec oneterm-mysql mysql -u root -p[OLD_PASSWORD] < backup_TIMESTAMP.sql
   
   # Start all services
   docker compose up -d
   ```

### ACL Service Connection Issues

If ACL service can't connect to database after password change:

1. Verify .env file contains correct database URL
2. Check ACL container logs for connection errors
3. Ensure the updated ACL image supports environment variable overrides

### Permission Denied Errors

If you get permission errors running scripts:

```bash
chmod +x setup.sh migrate-passwords.sh
```

## Advanced Configuration

### Custom Database Configuration

You can manually edit configuration files if needed:

1. **docker-compose.yaml**: MySQL root password
2. **config.yaml**: OneTerm database connection
3. **create-users.sql**: Database user creation
4. **.env**: ACL service database configuration

### Using External Database

To use an external MySQL database:

1. Update database host in config.yaml and .env
2. Ensure network connectivity
3. Create required databases and users manually
4. Skip MySQL container in docker-compose.yaml

## Support

For issues or questions:

1. Check the main OneTerm documentation
2. Review container logs for error messages
3. Ensure all prerequisites are met
4. Verify network connectivity between containers

## Version Compatibility

These scripts are designed for OneTerm v25.8.2 later. For older versions, manual configuration may be required.