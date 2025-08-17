#!/bin/bash
# migrate-passwords.sh - Password migration tool for existing OneTerm installations
# This script safely migrates database passwords for running OneTerm systems

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${CYAN}=== OneTerm Password Migration Tool ===${NC}"
echo -e "${YELLOW}WARNING: This will change database passwords for running system${NC}"
echo -e "${BLUE}Please ensure you have backups before proceeding${NC}"
echo

# Function to check if docker compose is available
check_docker_compose() {
    if command -v docker-compose &> /dev/null; then
        DOCKER_COMPOSE="docker-compose"
    elif docker compose version &> /dev/null; then
        DOCKER_COMPOSE="docker compose"
    else
        echo "ERROR: Docker Compose not found"
        exit 1
    fi
}

# Function to verify container is running
check_container_running() {
    local container_name=$1
    if ! docker ps --format "table {{.Names}}" | grep -q "^${container_name}$"; then
        echo -e "${RED}ERROR: Container ${container_name} is not running${NC}"
        echo "Please start OneTerm containers first: ${DOCKER_COMPOSE} up -d"
        exit 1
    fi
}

# Function to test database connection
test_db_connection() {
    local username=$1
    local password=$2
    local database=${3:-""}
    
    if [ -n "$database" ]; then
        docker exec oneterm-mysql mysql -u "$username" -p"$password" "$database" -e "SELECT 1;" &>/dev/null
    else
        docker exec oneterm-mysql mysql -u "$username" -p"$password" -e "SELECT 1;" &>/dev/null
    fi
}

# Initialize
check_docker_compose

echo "Checking system status..."
check_container_running "oneterm-mysql"

# Get current passwords
echo "Enter CURRENT passwords (press Enter for default '123456'):"
echo -n "Current MySQL root password [123456]: "
read -s CURRENT_ROOT
CURRENT_ROOT=${CURRENT_ROOT:-123456}
echo

echo -n "Current ACL database password [123456]: "
read -s CURRENT_ACL  
CURRENT_ACL=${CURRENT_ACL:-123456}
echo

echo -n "Current OneTerm database password [123456]: "
read -s CURRENT_ONETERM
CURRENT_ONETERM=${CURRENT_ONETERM:-123456}
echo

# Get new passwords
echo
echo "Enter NEW passwords:"
echo -n "New MySQL root password: "
read -s NEW_ROOT
echo

while [ -z "$NEW_ROOT" ]; do
    echo "Root password cannot be empty!"
    echo -n "New MySQL root password: "
    read -s NEW_ROOT
    echo
done

echo -n "New ACL database password: "
read -s NEW_ACL
echo

while [ -z "$NEW_ACL" ]; do
    echo "ACL password cannot be empty!"
    echo -n "New ACL database password: "
    read -s NEW_ACL
    echo
done

echo -n "New OneTerm database password: "
read -s NEW_ONETERM
echo

while [ -z "$NEW_ONETERM" ]; do
    echo "OneTerm password cannot be empty!"
    echo -n "New OneTerm database password: "
    read -s NEW_ONETERM
    echo
done

# Verify current passwords
echo
echo "Verifying current passwords..."
if ! test_db_connection "root" "$CURRENT_ROOT"; then
    echo "ERROR: Current root password is incorrect!"
    exit 1
fi

if ! test_db_connection "acl" "$CURRENT_ACL" "acl"; then
    echo "ERROR: Current ACL password is incorrect!"
    exit 1
fi

if ! test_db_connection "oneterm" "$CURRENT_ONETERM" "oneterm"; then
    echo "ERROR: Current OneTerm password is incorrect!"
    exit 1
fi

echo -e "${GREEN}SUCCESS: Current passwords verified${NC}"

# Create backup
echo
echo "Creating database backup..."
BACKUP_FILE="backup_$(date +%Y%m%d_%H%M%S).sql"
docker exec oneterm-mysql mysqldump -u root -p"${CURRENT_ROOT}" --all-databases > "$BACKUP_FILE"

if [ $? -eq 0 ]; then
    echo "SUCCESS: Database backup created: $BACKUP_FILE"
else
    echo "ERROR: Failed to create backup"
    exit 1
fi

# Stop application services (keep database running)
echo
echo "Stopping application services..."
$DOCKER_COMPOSE stop oneterm-api oneterm-acl-api oneterm-ui oneterm-guacd 2>/dev/null || true

# Wait a moment for graceful shutdown
sleep 5

# Update database passwords
echo
echo "Updating database passwords..."

# Update database passwords using direct execution
echo "Updating root password..."
docker exec oneterm-mysql mysql -u root -p"${CURRENT_ROOT}" -e "ALTER USER 'root'@'localhost' IDENTIFIED BY '${NEW_ROOT}'; ALTER USER 'root'@'%' IDENTIFIED BY '${NEW_ROOT}'; FLUSH PRIVILEGES;" 2>/dev/null

if [ $? -eq 0 ]; then
    echo -e "${GREEN}Root password updated successfully${NC}"
else
    echo "ERROR: Failed to update root password"
    exit 1
fi

echo "Updating application user passwords..."
docker exec oneterm-mysql mysql -u root -p"${NEW_ROOT}" -e "ALTER USER 'acl'@'%' IDENTIFIED BY '${NEW_ACL}'; ALTER USER 'oneterm'@'%' IDENTIFIED BY '${NEW_ONETERM}'; FLUSH PRIVILEGES;" 2>/dev/null

if [ $? -eq 0 ]; then
    echo -e "${GREEN}SUCCESS: Database passwords updated${NC}"
else
    echo "ERROR: Failed to update application user passwords"
    echo "You may need to restore from backup: $BACKUP_FILE"
    exit 1
fi

# Backup configuration files
echo
echo "Backing up configuration files..."
cp docker-compose.yaml docker-compose.yaml.backup.$(date +%Y%m%d_%H%M%S) 2>/dev/null || true
cp config.yaml config.yaml.backup.$(date +%Y%m%d_%H%M%S) 2>/dev/null || true
cp .env .env.backup.$(date +%Y%m%d_%H%M%S) 2>/dev/null || true

# Update docker-compose.yaml
echo "Updating docker-compose.yaml..."
if [ -f docker-compose.yaml ]; then
    # Create a temporary file for safer processing
    cp docker-compose.yaml docker-compose.yaml.tmp
    
    # Escape special characters in password for sed
    ESCAPED_NEW_ROOT=$(printf '%s\n' "$NEW_ROOT" | sed 's/[[\.*^$()+?{|]/\\&/g')
    
    # Update MySQL root password - use escaped password
    sed -i '' "s|MYSQL_ROOT_PASSWORD: '[^']*'|MYSQL_ROOT_PASSWORD: '${ESCAPED_NEW_ROOT}'|g" docker-compose.yaml.tmp
    sed -i '' "s|MYSQL_ROOT_PASSWORD: \"[^\"]*\"|MYSQL_ROOT_PASSWORD: \"${ESCAPED_NEW_ROOT}\"|g" docker-compose.yaml.tmp
    
    # Handle unquoted passwords
    sed -i '' "s|MYSQL_ROOT_PASSWORD: [^'\"[:space:]]*$|MYSQL_ROOT_PASSWORD: '${ESCAPED_NEW_ROOT}'|g" docker-compose.yaml.tmp
    
    # Update healthcheck password for MySQL - handle current format
    sed -i '' "s|\"-p[0-9][0-9]*\"|\"-p${ESCAPED_NEW_ROOT}\"|g" docker-compose.yaml.tmp
    
    # Move updated file back
    mv docker-compose.yaml.tmp docker-compose.yaml
    
    echo "SUCCESS: docker-compose.yaml updated"
fi

# Update config.yaml for OneTerm (only MySQL password, not Redis)
echo "Updating config.yaml..."
if [ -f config.yaml ]; then
    # config.yaml uses root user, so use root password
    ESCAPED_NEW_ROOT_FOR_CONFIG=$(printf '%s\n' "$NEW_ROOT" | sed 's/[[\.*^$()+?{|]/\\&/g')
    # Update only MySQL password section, not Redis
    sed -i .tmp '/^mysql:/,/^[a-z]/s|password: [^#]*|password: '"${ESCAPED_NEW_ROOT_FOR_CONFIG}"'|g' config.yaml
    rm -f config.yaml.tmp
    echo -e "${GREEN}SUCCESS: config.yaml updated${NC}"
fi

# Update .env file for ACL
echo "Updating .env file..."
cat > .env << EOF
# Updated by migration script $(date)
FLASK_APP=autoapp.py
FLASK_DEBUG=1
FLASK_ENV=development
GUNICORN_WORKERS=2
LOG_LEVEL=debug
SECRET_KEY='xW2FAUfgffjmerTEBXADmURDOQ43ojLN'

# Database password - used by settings.py to build connection string
DB_ACL_PASSWORD=${NEW_ACL}
DB_ONETERM_PASSWORD=${NEW_ONETERM}
DB_ROOT_PASSWORD=${NEW_ROOT}
EOF

echo -e "${GREEN}SUCCESS: .env file updated${NC}"

# Note: ACL API settings.py should be updated in the Docker image to read DB_ACL_PASSWORD from environment

# Restart services
echo
echo "Restarting OneTerm services..."
$DOCKER_COMPOSE down
$DOCKER_COMPOSE up -d

# Wait for services to start
echo "Waiting for services to start..."
sleep 30

# Health check
echo
echo "Performing health check..."
UNHEALTHY_SERVICES=0

# Check each service
for service in oneterm-mysql oneterm-redis oneterm-acl-api oneterm-api oneterm-ui; do
    if docker ps --format "table {{.Names}}\t{{.Status}}" | grep "$service" | grep -q "unhealthy\|Exited\|Restarting"; then
        echo "WARNING: Service $service appears unhealthy"
        UNHEALTHY_SERVICES=$((UNHEALTHY_SERVICES + 1))
    else
        echo "OK: Service $service is running"
    fi
done

# Test database connections with new passwords
echo
echo "Testing database connections..."
if test_db_connection "root" "$NEW_ROOT"; then
    echo "OK: Root connection successful"
else
    echo "WARNING: Root connection failed"
    UNHEALTHY_SERVICES=$((UNHEALTHY_SERVICES + 1))
fi

if test_db_connection "acl" "$NEW_ACL" "acl"; then
    echo "OK: ACL connection successful"
else
    echo "WARNING: ACL connection failed"
    UNHEALTHY_SERVICES=$((UNHEALTHY_SERVICES + 1))
fi

if test_db_connection "oneterm" "$NEW_ONETERM" "oneterm"; then
    echo "OK: OneTerm connection successful"
else
    echo "WARNING: OneTerm connection failed"
    UNHEALTHY_SERVICES=$((UNHEALTHY_SERVICES + 1))
fi

# Final status
echo
echo -e "${CYAN}=== Migration Summary ===${NC}"
if [ $UNHEALTHY_SERVICES -eq 0 ]; then
    echo -e "${GREEN}SUCCESS: Password migration completed successfully!${NC}"
    echo -e "${GREEN}All services are running normally with new passwords.${NC}"
else
    echo -e "${YELLOW}WARNING: Migration completed but $UNHEALTHY_SERVICES issue(s) detected.${NC}"
    echo -e "${BLUE}Check service logs if you experience problems:${NC}"
    echo -e "  ${CYAN}$DOCKER_COMPOSE logs oneterm-api${NC}"
    echo -e "  ${CYAN}$DOCKER_COMPOSE logs oneterm-acl-api${NC}"
    echo -e "  ${CYAN}$DOCKER_COMPOSE logs oneterm-mysql${NC}"
fi

echo
echo "Backup files created:"
echo "  Database backup: $BACKUP_FILE"
echo "  Config backups: *.backup.*"
echo
echo "If you encounter issues, you can:"
echo "1. Check logs: $DOCKER_COMPOSE logs [service-name]"
echo "2. Restart services: $DOCKER_COMPOSE restart"
echo "3. Restore from backup if needed"
echo
echo "Migration completed at $(date)"
