<h3 align="center">oneterm</h3>
<p align="center">
  <a href="https://github.com/veops/oneterm/blob/main/LICENSE"><img src="https://img.shields.io/github/license/veops/oneterm" alt="Apache License 2.0"></a>
  <a href=""><img src="https://img.shields.io/badge/Go-%3E%3D%201.18-%23007d9c" alt="go>=1.18"></a>
  <a href="https://goreportcard.com/report/github.com/veops/oneterm"><img src="https://goreportcard.com/badge/github.com/veops/oneterm" alt="API"></a>
</p>
oneterm 简单、轻量、安全的跳板机服务

---

## 🚀安装

### docker-compose

```bash
cd oneterm
cp cmd/api/confTemplate.yaml cmd/api/conf.yaml # edit your config
cp cmd/ssh/confTemplate.yaml cmd/ssh/conf.yaml # edit your config
docker compose up -d
```

## 👀在线体验

在线地址：https://demo.veops.com/

账号：admin/user

密码：admin/user


## 📚产品文档

文档地址：https://veops.cn/docs/

## 🔗相关项目

[go-ansiterm](https://github.com/veops/go-ansiterm)：linux终端仿真器

## 🤝社区交流

**欢迎关注公众号(维易科技OneOps)，关注后可加入微信群，进行产品和技术交流。**

![公众号: 维易科技OneOps](docs/images/wechat.jpg)