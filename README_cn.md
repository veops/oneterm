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
 一款简单、轻量、灵活的堡垒机服务.
</p>

---
[English](README.md) / 中文
- 产品文档：https://veops.cn/docs/
- 在线体验: <a href="https://term.veops.cn/oneterm/workstation" target="_blank">OneTerm</a>
    - username: **`demo`**   或者   **`admin`**
    - password: **`123456`**
> **重要提示**:  **`main`** 分支在开发过程中可能处于不稳定的状态，请通过[release](https://github.com/veops/oneterm/releases)获取，或者直接通过镜像部署


## 🚀安装

### docker-compose

```bash
git clone https://github.com/veops/oneterm.git
cd oneterm
docker compose up -d
```

## ✅验证
- 浏览器: [http://127.0.0.1:8666](http://127.0.0.1:8666)
- 账号: admin
- 密码: 123456

## 🎯功能

- 资产管理 (SSH RDP VNC)
- 账号管理
- 权限认证
- 会话管理
  - 在线会话: 监控、强制关闭
  - 离线会话: 回放, 下载
- SSH服务
- 资产账号自动发现

## 📚产品文档

文档地址：https://veops.cn/docs/docs/oneterm/onterm_design


## 如何贡献

<a href="https://github.com/veops/oneterm/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=veops/oneterm" />
</a>

对于那些想要贡献代码的人，请参阅我们的[贡献指南](CONTRIBUTING.md)。
同时，请考虑通过社交媒体、活动和分享来支持 Veops 的开源。

## 🤝社区交流

**欢迎关注公众号(维易科技OneOps)，关注后可加入微信群，进行产品和技术交流。**

<p align="center">
  <img src="docs/images/wechat.png" alt="公众号: 维易科技OneOps" />
</p>