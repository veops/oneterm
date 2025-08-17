# OneTerm Development Environment Setup Guide

> **Language**: [English](DEV_README.md) | [中文](DEV_README.zh.md)

This guide helps developers quickly set up OneTerm's development environment for independent frontend and backend development.

## Environment Options

### 🎨 Frontend Development Environment
**For**: Vue.js frontend development, UI debugging, frontend feature development
- **Containers**: MySQL, Redis, ACL-API, Guacd, OneTerm-API (optional)
- **Local**: Frontend project

### ⚙️ Backend Development Environment  
**For**: Go backend development, API development, protocol connector development
- **Containers**: MySQL, Redis, ACL-API, Guacd, OneTerm-UI
- **Local**: Backend project

## Quick Start

### Prerequisites
- Docker & Docker Compose
- Node.js 14.17.6+ (for frontend development)
- Go 1.21.3+ (for backend development)
- Git

### 1. Clone the Project
```bash
git clone <your-repo-url>
cd oneterm
```

### 2. Choose Your Development Environment

#### 🎨 Frontend Development Environment

1. **Start Backend Dependencies**
```bash
cd deploy
# Start necessary backend services
docker compose -f docker-compose.frontend-dev.yaml up -d

# Check service status
docker compose -f docker-compose.frontend-dev.yaml ps
```

2. **Run Frontend Locally**
```bash
cd oneterm-ui

# Install dependencies
yarn install

# Start development server
npm run serve
```

3. **Access the Application**
- Frontend dev server: http://localhost:8000
- OneTerm API: http://localhost:18888
- ACL API: http://localhost:15000

#### ⚙️ Backend Development Environment

1. **Start Frontend and Dependencies**
```bash
cd deploy
# Start frontend and necessary services
docker compose -f docker-compose.backend-dev.yaml up -d

# Check service status
docker compose -f docker-compose.backend-dev.yaml ps
```

2. **Configure Backend**
```bash
cd backend/cmd/server
# Copy development configuration (pre-configured for dev environment)
cp ../../deploy/dev-config.example.yaml config.yaml
```

3. **Run Backend Locally**
```bash
cd backend/cmd/server

# Install dependencies
go mod tidy

# Run server
go run main.go config.yaml
```

4. **Access the Application**
- Frontend UI: http://localhost:8666
- Backend API: http://localhost:8888
- SSH port: localhost:2222

## Development Workflow

### Frontend Development

```bash
# Development
cd oneterm-ui
npm run serve          # Start development server
npm run lint           # Code linting
npm run lint:nofix     # Check only without fixing

# Build
npm run build          # Production build
npm run build:preview  # Preview build
```

### Backend Development

```bash
# Development
cd backend/cmd/server
go run main.go config.yaml     # Run server

# Build and test
cd backend
go mod tidy                    # Update dependencies
go build ./...                 # Build all packages
go test ./...                  # Run tests

# Production build
cd backend/cmd/server
./build.sh                     # Build Linux binaries
```

## Database Management

### Connection Info
- **MySQL**: localhost:13306
- **Username**: root
- **Password**: 123456
- **Databases**: oneterm, acl

### Common Operations
```bash
# Connect to MySQL
mysql -h localhost -P 13306 -u root -p123456

# View databases
show databases;
use oneterm;
show tables;

# Reset database (use with caution)
cd deploy
docker compose -f docker-compose.frontend-dev.yaml down -v
docker compose -f docker-compose.frontend-dev.yaml up -d
```

## Troubleshooting

### Port Conflicts
If you encounter port conflicts, modify the port mappings in docker-compose files:
```yaml
ports:
  - "new-port:container-port"
```

### Database Connection Failed
1. Ensure MySQL container is started and healthy
2. Check database connection parameters in config file
3. Verify port mappings are correct

### Frontend Proxy Issues
Check proxy configuration in `oneterm-ui/vue.config.js`:
```javascript
devServer: {
  proxy: {
    '/api': {
      target: 'http://localhost:18888',  // Ensure correct backend address
      changeOrigin: true
    }
  }
}
```

### ACL Permission Issues
1. Ensure ACL-API service is running normally
2. Check if initialization is complete
3. View container logs: `docker logs oneterm-acl-api-dev`

## Quick Start Script

Use the convenient startup script:

```bash
cd deploy

# Frontend development mode
./dev-start.sh frontend

# Backend development mode
./dev-start.sh backend

# Full environment mode
./dev-start.sh full

# Stop all services
./dev-start.sh stop

# Show help
./dev-start.sh help
```

## Environment Cleanup

```bash
# Stop development environment
cd deploy
docker compose -f docker-compose.frontend-dev.yaml down
# or
docker compose -f docker-compose.backend-dev.yaml down

# Clean all data (including database)
docker compose -f docker-compose.frontend-dev.yaml down -v
```

## Debugging Tips

### View Logs
```bash
# View all service logs
docker compose -f docker-compose.frontend-dev.yaml logs

# View specific service logs
docker compose -f docker-compose.frontend-dev.yaml logs mysql
docker compose -f docker-compose.frontend-dev.yaml logs acl-api

# Follow logs in real-time
docker compose -f docker-compose.frontend-dev.yaml logs -f
```

### Enter Containers
```bash
# Enter MySQL container
docker exec -it oneterm-mysql-dev bash

# Enter ACL-API container
docker exec -it oneterm-acl-api-dev bash
```

## Configuration Files

### Backend Configuration
Use the development configuration template:
```bash
cd backend/cmd/server
cp ../../deploy/dev-config.example.yaml config.yaml
# Configuration is pre-configured for development environment
```

### Frontend Configuration
The frontend automatically proxies to the backend. For custom proxy settings, edit `oneterm-ui/vue.config.js`.

## Contributing

+ [CONTRIBUTING.md](../CONTRIBUTING.md)

## Support

- Project Documentation: See README.md in project root
- Issue Reporting: Create GitHub Issues
- Development Discussion: Participate in project discussions

---

**Happy Coding! 🚀**