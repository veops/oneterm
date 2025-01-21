<p align="center">
  <img src="https://github.com/user-attachments/assets/ab00344b-462b-44b9-9113-9fe735dfb096" />
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

OneTerm is a simple, lightweight and flexible enterprise-class bastion host, designed and developed based on 4A compliant, i.e. Authen, Authorize, Account, and Audit, which ensures the security and compliance of the system through strict access control and monitoring features.

- Product document：https://veops.cn/docs/docs/oneterm/onterm_design
- Preview online：[OneTerm](https://term.veops.cn/oneterm/workstation)
  - username: **demo** or **admin**
  - password: 123456
- **ATTENTION**: branch `main` may be unstable as the result of continued development, Please use [releases](https://github.com/veops/oneterm/releases) to get the latest stable version

## Core Feature

+ **Access control**: Acting as an intermediary, OneTerm restricts direct access to critical systems. Users must authenticate through OneTerm before accessing other servers or systems.

+ **Security audit**: OneTerm can record user logins and activities, providing audit logs for investigation in case of security incidents. This ensures that every user's actions are traceable and auditable.

+ **Jump access to**: OneTerm offers a jump host mechanism, allowing users to connect to other internal servers through OneTerm. This helps reduce the risk of exposing internal servers directly to the outside, as only OneTerm needs to be accessible externally.

+ **Password management**: OneTerm can enforce robust password policies and centrally manage passwords through a single entry point. This helps improve the overall system's password security.

+ **Session recording**: OneTerm can record user sessions with servers, which is valuable for monitoring and investigating privileged user activities. In case of security incidents, session recordings can be replayed to understand detailed operations.

+ **Prevent direct attacks**: Since OneTerm is the sole entry point for systems and resources, it can serve as a primary obstacle for attackers. This helps reduce the risk of direct attacks on internal systems.

+ **Unified access**: OneTerm provides a single entry point through which users can access different systems without needing to remember multiple login credentials. This enhances user convenience and work efficiency.

## Product Advantage

+ **Authentication and Authorization**: Authentication and Authorization: OneTerm should have a robust and flexible identity authentication and authorization mechanism. This includes supporting multi-factor authentication to ensure that only authorized users can access internal network resources and enabling fine-grained management of user permissions. 
+ **Secure communication**: OneTerm supports secure communication protocols and encryption technologies to protect data transmission between users and internal servers. This helps prevent man-in-the-middle attacks and data leakage. 
+ **Audit and monitoring**: OneTerm features powerful audit and monitoring capabilities, recording user activities and generating audit logs. This helps trace security incidents, identify potential threats, and meet compliance requirements. 
+ **Remote Management and Session Isolation**: OneTerm supports remote management, allowing administrators to securely manage internal servers. Additionally, it should have session isolation functionality to ensure that access between users is isolated from each other, preventing lateral movement attacks. 
+ **Combination with open source CMDB**: Oneterm is combined with [VE CMDB](https://github.com/veops/cmdb) (which has been open source), users can import assets in CMDB with one click, ensuring easy operation and smooth process.

## Tech Stack

+ Back-end: Go
+ Front-end: Vue.js
+ UI component library: Ant Design Vue

## Getting started & staying tuned with us

Star us, and you will receive all releases notifications from GitHub without any delay!

![star us](https://github.com/user-attachments/assets/75c03659-4200-469e-b210-087a4d4473b6)

## Overview

<table>
  <tr>
    <td style="padding: 5px;background-color:#fff;">
      <img width="400" src="https://github.com/user-attachments/assets/abefbe07-13d6-44b0-8622-a0c7130d5b0d"/>
    </td>
    <td style="padding: 5px;background-color:#fff;">
      <img width="400" src="https://github.com/user-attachments/assets/3a69c779-3f37-4c5b-8ade-2dffa99a2efd"/>
    </td>
  </tr>

  <tr>
    <td style="padding: 5px;background-color:#fff;">
      <img width="400" src="https://github.com/user-attachments/assets/befcfae7-f24a-48a2-a730-8e8d02483ea9"/>
    </td>
    <td style="padding: 5px;background-color:#fff;">
      <img width="400" src="https://github.com/user-attachments/assets/75d33250-af61-4c22-b839-cd6ba9ecd551"/>
    </td>
  </tr>
</table>

## Quick Start

+ docker-compose install
  ```bash
  git clone https://github.com/veops/oneterm.git
  cd oneterm
  docker compose up -d
  ```
+ visit
  - Open your browser and visit: [http://127.0.0.1:8666](http://127.0.0.1:8666)
  - Username: admin
  - Password: 123456

## Contributing

We welcome all developers to contribute code to improve and extend this project. Please read our [contribution guidelines](CONTRIBUTING.md) first. Additionally, you can support Veops open source through social media, events, and sharing.

<a href="https://github.com/veops/oneterm/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=veops/oneterm" />
</a>

## More Open Source
- [CMDB](https://github.com/veops/cmdb): Simple, lightweight, and versatile operational CMDB
- [ACL](https://github.com/veops/acl): A general permission control management system.
- [messenger](https://github.com/veops/messenger): A simple and lightweight message sending service.

## Community

+ Email: <a href="mailto:bd@veops.cn">bd@veops.cn</a>
+ WeChat official account: Welcome to follow our WeChat official account and join our group channels
  <img src="docs/images/wechat.png" alt="WeChat official account" />