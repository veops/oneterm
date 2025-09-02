<p align="center">
  <a href="https://v1ops.com/">
    <img alt="oneterm_banner" src="https://github.com/user-attachments/assets/6a96c210-3c85-4b8e-ad84-6cecd95e2066" />
  </a>
</p>

<p align="center">
  <a href="https://github.com/veops/oneterm/blob/main/LICENSE"><img src="https://img.shields.io/github/license/veops/oneterm" alt="Apache License 2.0"></a>
  <a href="https://github.com/veops/oneterm/releases"><img alt="the latest release version" src="https://img.shields.io/github/v/release/veops/oneterm?color=75C1C4&include_prereleases&label=Release&logo=github&logoColor=white"></a>
  <a href=""><img src="https://img.shields.io/badge/Go-%3E%3D%201.18-%23007d9c" alt="go>=1.18"></a>
  <a href="https:https://github.com/sendya/ant-design-pro-vue"><img src="https://img.shields.io/badge/UI-Ant%20Design%20Pro%20Vue-brightgreen" alt="UI"></a>
  <a href="https://github.com/veops/oneterm/stargazers"><img src="https://img.shields.io/github/stars/veops/oneterm" alt="Stars Badge"/></a>
  <a href="https://github.com/veops/oneterm"><img src="https://img.shields.io/github/forks/veops/oneterm" alt="Forks Badge"/></a>
</p>

<h4 align="center">
 A Simple, Lightweight, Flexible Bastion Host.
</h4>

<p align="center">
  <a href="https://trendshift.io/repositories/8690" target="_blank"><img src="https://trendshift.io/api/badge/repositories/8690" alt="veops%2Foneterm | Trendshift" style="width: 250px; height: 55px;" width="250" height="55"/></a>
</p>

<p align="center">
  English · <a href="README_cn.md">中文(简体)</a>
</p>

## What is OneTerm

OneTerm is a simple, lightweight, and flexible enterprise-level bastion host product. Based on the 4A concept: Authentication, Authorization, Account, and Audit, it ensures system security and compliance through strict access control and monitoring functions.

- Official Website: [v1ops.com](https://v1ops.com/)
- Product Documentation: [v1ops.com/docs/design](https://v1ops.com/docs/design/)
- Online Demo: [oneterm.v1ops.com](https://oneterm.v1ops.com/)
  - Username: demo or admin
  - Password: 123456
- **Note**: The `main` branch may be in an **unstable state** during development. Please obtain the latest stable version through [releases](https://github.com/veops/oneterm/releases).

## Core Features

+ **Access Control**: OneTerm acts as an intermediary site, restricting direct access to critical systems. Users must first authenticate through OneTerm before accessing other servers or systems.

+ **Security Audit**: OneTerm can record user logins and activities, providing audit logs for investigation when security incidents occur. This helps ensure that every user's behavior is traceable and auditable.

+ **Jump Server Access**: OneTerm provides a jump server approach where users can connect to other internal servers through OneTerm. This approach helps reduce the risk of directly exposing internal servers, as only OneTerm needs to be externally accessible.

+ **Password Management**: OneTerm can implement enhanced password policies and centrally manage passwords through a single entry point. This helps improve the password security of the entire system.

+ **Session Recording**: OneTerm can record user sessions with servers, which is very useful for monitoring and investigating privileged user activities. If security incidents occur, session recordings can be replayed to understand detailed operations.

+ **Prevent Direct Attacks**: Since OneTerm is the only entry point to systems and resources, it can become the main barrier for attackers. This helps reduce the risk of direct attacks on internal systems.

+ **Unified Access**: OneTerm provides a single entry point through which users can access different systems without having to remember multiple login credentials. This improves user convenience and work efficiency.

## Product Advantages

+ **Authentication and Authorization**: OneTerm features powerful and flexible authentication and authorization mechanisms. This includes support for multi-factor authentication, ensuring that only authorized users can access internal network resources, and providing fine-grained user permission management.

+ **Secure Communication**: OneTerm supports secure communication protocols and encryption technologies to protect data transmission between users and internal servers. This helps prevent man-in-the-middle attacks and data leaks.

+ **Audit and Monitoring**: OneTerm has powerful audit and monitoring capabilities, recording user activities and generating audit logs. This helps track security events, identify potential threats, and meet compliance requirements.

+ **Remote Management and Session Isolation**: OneTerm supports remote management, enabling administrators to securely manage internal servers. At the same time, it features session isolation to ensure that access between users is mutually isolated, preventing lateral escalation attacks.

+ **Tight Integration with Open Source CMDB**: OneTerm is tightly integrated with [Veops CMDB](https://github.com/veops/cmdb) (open source), allowing users to import assets from CMDB with one click, ensuring convenient operation and smooth processes.

## Technology Stack

+ Backend: Go
+ Frontend: Vue.js
+ UI Component Library: Ant Design Vue

## Follow Us

Welcome to Star and follow us to get the latest updates!

![star us](https://github.com/user-attachments/assets/75c03659-4200-469e-b210-087a4d4473b6)

## Project Overview

<table>
  <tr>
    <td style="padding: 5px;background-color:#fff;">
      <img width="400" alt="dashboard" src="https://github.com/user-attachments/assets/cfbb7ae9-ddd3-4f0f-a37b-18b68bd8c7ac" />
    </td>
    <td style="padding: 5px;background-color:#fff;">
      <img width="400" alt="terminal" src="https://github.com/user-attachments/assets/e37f0ce8-d07c-42e0-a603-028b75c8e932" />
    </td>
  </tr>

  <tr>
    <td style="padding: 5px;background-color:#fff;">
      <img width="400" alt="work_station" src="https://github.com/user-attachments/assets/48a11f13-88be-4ec1-aa06-fbaade5721b1" />
    </td>
    <td style="padding: 5px;background-color:#fff;">
      <img width="400" alt="access_auth" src="https://github.com/user-attachments/assets/d1db6c5f-3ac0-46a1-9464-34c4c59243ed" />
    </td>
  </tr>

  <tr>
    <td style="padding: 5px;background-color:#fff;">
      <img width="400" alt="system_settings" src="https://github.com/user-attachments/assets/b9948d82-071a-427b-884f-a69fd37b27ae" />
    </td>
    <td style="padding: 5px;background-color:#fff;">
      <img width="400" alt="access_time" src="https://github.com/user-attachments/assets/55298679-8ded-4948-9738-38e418c2d03d" />
    </td>
  </tr>
</table>

## Quick Start

### Method 1: Quick Deploy (Default Password)
+ Docker Compose Installation
  ```bash
  git clone https://github.com/veops/oneterm.git
  cd oneterm/deploy
  docker compose up -d
  ```

### Method 2: Secure Deploy (Custom Passwords)
+ For production environments, use the setup script to configure secure passwords:
  ```bash
  git clone https://github.com/veops/oneterm.git
  cd oneterm/deploy
  ./setup.sh
  docker compose up -d
  ```
  The setup script will:
  - Generate secure random passwords or let you set custom ones
  - Update all configuration files with your passwords
  - Create backup files for safety

+ **Access**
  - Open your browser and visit: [http://127.0.0.1:8666](http://127.0.0.1:8666)
  - Username: admin
  - Password: 123456 (default) or your custom password if using setup.sh

## Development

For developers who want to contribute to OneTerm or set up a local development environment:

### 🚀 Quick Development Setup
```bash
# Clone repository
git clone https://github.com/veops/oneterm.git
cd oneterm/deploy

# Frontend development (live editing)
./dev-start.sh frontend

# Backend development (live editing)  
./dev-start.sh backend

```

### 📖 Detailed Development Guide
For complete setup instructions, troubleshooting, and development workflows:
- **[Development Environment Setup Guide](deploy/DEV_README.md)**

**Requirements**: Docker, Node.js 14.17.6+, Go 1.21.3+

## Contributing
We welcome all developers to contribute code and improve and extend this project. Please read our [Contribution Guide](CONTRIBUTING.md) first. Additionally, you can support Veops open source through social media, events, and sharing.

<a href="https://github.com/veops/oneterm/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=veops/oneterm" />
</a>

## More Open Source
- [CMDB](https://github.com/veops/cmdb): Simple, lightweight, and versatile operational CMDB
- [ACL](https://github.com/veops/acl): A general permission control management system.
- [messenger](https://github.com/veops/messenger): A simple and lightweight message sending service.

## Contact Us
+ Email: <a href="mailto:bd@veops.cn">bd@veops.cn</a>
