#!/bin/bash
# setup.sh - Initial setup script for new OneTerm installations
# This script helps users configure passwords before first deployment

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${CYAN}=== OneTerm Initial Setup ===${NC}"
echo -e "${BLUE}This script will help you configure OneTerm for first-time deployment${NC}"
echo

# Function to generate random password
generate_password() {
    openssl rand -base64 12 2>/dev/null || date +%s | sha256sum | base64 | head -c 12
}

# Function to check if files exist
check_files() {
    local missing_files=0
    
    if [ ! -f "docker-compose.yaml" ]; then
        echo "ERROR: docker-compose.yaml not found in current directory"
        missing_files=$((missing_files + 1))
    fi
    
    if [ ! -f "config.yaml" ]; then
        echo "ERROR: config.yaml not found in current directory"
        missing_files=$((missing_files + 1))
    fi
    
    if [ ! -f "create-users.sql" ]; then
        echo "ERROR: create-users.sql not found in current directory"
        missing_files=$((missing_files + 1))
    fi
    
    if [ $missing_files -gt 0 ]; then
        echo "Please run this script from the OneTerm deploy directory"
        exit 1
    fi
}

# Function to backup original files
backup_files() {
    local timestamp=$(date +%Y%m%d_%H%M%S)
    
    echo "Creating backup of original files..."
    cp docker-compose.yaml docker-compose.yaml.original.$timestamp
    cp config.yaml config.yaml.original.$timestamp
    cp create-users.sql create-users.sql.original.$timestamp
    echo "Backup files created with suffix: .original.$timestamp"
}

# Function to validate password strength
validate_password() {
    local password=$1
    local min_length=8
    
    if [ ${#password} -lt $min_length ]; then
        echo "Password must be at least $min_length characters long"
        return 1
    fi
    
    return 0
}

# Cross-platform sed function
cross_platform_sed() {
    local pattern="$1"
    local file="$2"
    
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' "$pattern" "$file"
    else
        sed -i "$pattern" "$file"
    fi
}


# Cross-platform sed function with backup
cross_platform_sed_backup() {
    local pattern="$1"
    local file="$2"
    local backup_suffix="$3"
    
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i ".${backup_suffix}" "$pattern" "$file"
    else
        sed -i".${backup_suffix}" "$pattern" "$file"
    fi
}

# Check prerequisites
check_files

# Check if this looks like an existing installation
if docker ps 2>/dev/null | grep -q "oneterm"; then
    echo "WARNING: OneTerm containers are already running!"
    echo "This appears to be an existing installation."
    echo "For existing installations, use migrate-passwords.sh instead"
    echo
    echo -n "Continue anyway? (y/N): "
    read -r response
    if [[ ! "$response" =~ ^[Yy]$ ]]; then
        echo "Setup cancelled"
        exit 0
    fi
fi

# Backup original files
backup_files

echo
echo "Password Configuration:"
echo "You can either:"
echo "1. Enter custom passwords"
echo "2. Generate random passwords automatically"
echo

echo -n "Choose option (1/2) [2]: "
read -r option
option=${option:-2}

if [ "$option" = "1" ]; then
    # Manual password entry
    echo
    echo "Enter passwords for database users:"
    
    echo -n "MySQL root password: "
    read -s ROOT_PASS
    echo
    while ! validate_password "$ROOT_PASS"; do
        echo -n "MySQL root password (min 8 chars): "
        read -s ROOT_PASS
        echo
    done
    
    echo -n "ACL database password: "
    read -s ACL_PASS
    echo
    while ! validate_password "$ACL_PASS"; do
        echo -n "ACL database password (min 8 chars): "
        read -s ACL_PASS
        echo
    done
    
    echo -n "OneTerm database password: "
    read -s ONETERM_PASS
    echo
    while ! validate_password "$ONETERM_PASS"; do
        echo -n "OneTerm database password (min 8 chars): "
        read -s ONETERM_PASS
        echo
    done
    
elif [ "$option" = "2" ]; then
    # Generate random passwords
    echo
    echo "Generating random passwords..."
    ROOT_PASS=$(generate_password)
    ACL_PASS=$(generate_password)
    ONETERM_PASS=$(generate_password)
    
    echo "Generated passwords:"
    echo "  MySQL root: $ROOT_PASS"
    echo "  ACL database: $ACL_PASS"  
    echo "  OneTerm database: $ONETERM_PASS"
    echo
    echo "IMPORTANT: Save these passwords securely!"
    echo
else
    echo "Invalid option selected"
    exit 1
fi

# Update docker-compose.yaml
echo
echo "Updating docker-compose.yaml..."
# Create a temporary file for safer processing
cp docker-compose.yaml docker-compose.yaml.tmp

# Escape special characters in password for sed
ESCAPED_ROOT_PASS=$(printf '%s\n' "$ROOT_PASS" | sed 's/[[\.*^$()+?{|]/\\&/g')

# Update MySQL root password patterns - handle various formats
cross_platform_sed "s|MYSQL_ROOT_PASSWORD: '[^']*'|MYSQL_ROOT_PASSWORD: '${ESCAPED_ROOT_PASS}'|g" docker-compose.yaml.tmp
cross_platform_sed "s|MYSQL_ROOT_PASSWORD: \"[^\"]*\"|MYSQL_ROOT_PASSWORD: \"${ESCAPED_ROOT_PASS}\"|g" docker-compose.yaml.tmp
cross_platform_sed "s|MYSQL_ROOT_PASSWORD: [^'\"[:space:]]*$|MYSQL_ROOT_PASSWORD: '${ESCAPED_ROOT_PASS}'|g" docker-compose.yaml.tmp

# Update healthcheck password
cross_platform_sed "s|\"-p[0-9][0-9]*\"|\"-p${ESCAPED_ROOT_PASS}\"|g" docker-compose.yaml.tmp

# Move updated file back
mv docker-compose.yaml.tmp docker-compose.yaml

# Update config.yaml (only MySQL password, not Redis)
echo "Updating config.yaml..."
# config.yaml uses root user, so use root password
ESCAPED_ROOT_PASS_FOR_CONFIG=$(printf '%s\n' "$ROOT_PASS" | sed 's/[[\.*^$()+?{|]/\\&/g')
# Only update MySQL password section, not Redis
cross_platform_sed_backup '/^mysql:/,/^[a-z]/s|password: [^#]*|password: '"${ESCAPED_ROOT_PASS_FOR_CONFIG}"'|g' config.yaml tmp
rm -f config.yaml.tmp

# Update create-users.sql
echo "Updating create-users.sql..."
# Escape passwords for sed
ESCAPED_ONETERM_PASS=$(printf '%s\n' "$ONETERM_PASS" | sed 's/[[\.*^$()+?{|]/\\&/g')
ESCAPED_ACL_PASS=$(printf '%s\n' "$ACL_PASS" | sed 's/[[\.*^$()+?{|]/\\&/g')
# Update passwords using | as delimiter
cross_platform_sed_backup "s|'oneterm'@'%' IDENTIFIED BY '[^']*'|'oneterm'@'%' IDENTIFIED BY '${ESCAPED_ONETERM_PASS}'|g" create-users.sql tmp
cross_platform_sed_backup "s|'acl'@'%' IDENTIFIED BY '[^']*'|'acl'@'%' IDENTIFIED BY '${ESCAPED_ACL_PASS}'|g" create-users.sql tmp
rm -f create-users.sql.tmp

# Create/update .env file for ACL
echo "Creating .env file..."
cat > .env << EOF
# OneTerm Configuration
# Generated by setup script on $(date)

FLASK_APP=autoapp.py
FLASK_DEBUG=1
FLASK_ENV=development
GUNICORN_WORKERS=2
LOG_LEVEL=debug
SECRET_KEY='xW2FAUfgffjmerTEBXADmURDOQ43ojLN'

# Database password - used by settings.py to build connection string
DB_ROOT_PASSWORD=${ROOT_PASS}
DB_ACL_PASSWORD=${ACL_PASS}
DB_ONETERM_PASSWORD=${ONETERM_PASS}
EOF

# Note: ACL API Docker image should be updated to read DB_ACL_PASSWORD from environment

# Save passwords to a secure file
echo "Creating password reference file..."
cat > .passwords << EOF
# OneTerm Database Passwords
# Generated on $(date)
# Keep this file secure and do not commit to version control

MySQL Root Password: ${ROOT_PASS}
ACL Database Password: ${ACL_PASS}
OneTerm Database Password: ${ONETERM_PASS}
EOF

chmod 600 .passwords

echo
echo -e "${GREEN}=== Setup Complete ===${NC}"
echo -e "${GREEN}Configuration files have been updated with your passwords.${NC}"
echo
echo -e "${BLUE}Files modified:${NC}"
echo -e "  ${CYAN}- docker-compose.yaml${NC} (MySQL root password)"
echo -e "  ${CYAN}- config.yaml${NC} (MySQL root password for OneTerm API)"
echo -e "  ${CYAN}- create-users.sql${NC} (ACL and OneTerm database user passwords)"
echo -e "  ${CYAN}- .env${NC} (ACL database configuration)"
echo
echo -e "${BLUE}Files created:${NC}"
echo -e "  ${YELLOW}- .passwords${NC} (password reference - keep secure!)"
echo
echo -e "${YELLOW}Next steps:${NC}"
echo -e "${GREEN}1.${NC} Review the configuration files"
echo -e "${GREEN}2.${NC} Start OneTerm: ${CYAN}docker compose up -d${NC}"
echo -e "${GREEN}3.${NC} Wait for all services to become healthy"
echo -e "${GREEN}4.${NC} Access the web interface"
echo
echo "To check service status: docker compose ps"
echo "To view logs: docker compose logs [service-name]"
echo
echo "SECURITY NOTE: The .passwords file contains sensitive information."
echo "Keep it secure and consider deleting it after noting the passwords elsewhere."