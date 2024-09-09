<h3 align="center">OneTerm</h3>

<p align="center">
  <a href="https://github.com/veops/oneterm/blob/main/LICENSE"><img src="https://img.shields.io/github/license/veops/oneterm" alt="Apache License 2.0"></a>
  <a href="https://github.com/veops/oneterm/releases">
    <img alt="the latest release version" src="https://img.shields.io/github/v/release/veops/oneterm?color=75C1C4&include_prereleases&label=Release&logo=github&logoColor=white">
  </a>
  <a href=""><img src="https://img.shields.io/badge/Go-%3E%3D%201.18-%23007d9c" alt="go>=1.18"></a>
  <a href="https:https://github.com/sendya/ant-design-pro-vue"><img src="https://img.shields.io/badge/UI-Ant%20Design%20Pro%20Vue-brightgreen" alt="UI"></a>
</p>

<p align="center">
 A Simple, Lightweight, Flexible Bastion Host.
</p>


---
English / [中文](README_cn.md)
- Product document：https://veops.cn/docs/
- Preview online: <a href="https://term.veops.cn/oneterm/workstation" target="_blank">OneTerm</a>
   - username: **`demo`**   or   **`admin`**
   - password: 123456

> **ATTENTION**: branch `main` may be unstable as the result of continued development, please pull code from [release](https://github.com/veops/oneterm/releases) or deploy via docker image

## 🚀Install

### docker-compose

```bash
git clone https://github.com/veops/oneterm.git
cd oneterm/deploy
docker compose up -d
```

## ✅ Validation

- View: [http://127.0.0.1:8666](http://127.0.0.1:8666)
- username: admin
- password: 123456


## 🎯Features

- Asset Managent (SSH RDP VNC)
- Account Management
- Authorization
- Session Management
  - Online Session: Monitor, Force Kill
  - Offline Session: Replay, Download
- SSH Server
- Asset & Account Auto Discovery


## 📚Docs

doc link：https://veops.cn/docs/docs/oneterm/onterm_design

## Contributing

<a href="https://github.com/veops/oneterm/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=veops/oneterm" />
</a>

For those who want to contribute code, see our [Contributing Guidelines](CONTRIBUTING.md).
In the meantime, please consider supporting Veops open source through social media, events, and sharing.

## 🤝Community

**Welcome to follow our WeChat official account and join our group channels**

<p align="center">
  <img src="docs/images/wechat.png" alt="公众号: 维易科技OneOps" />
</p>