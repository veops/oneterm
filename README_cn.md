<h3 align="center">OneTerm</h3>
<p align="center">
  <a href="https://github.com/veops/oneterm/blob/main/LICENSE"><img src="https://img.shields.io/github/license/veops/oneterm" alt="Apache License 2.0"></a>
  <a href=""><img src="https://img.shields.io/badge/Go-%3E%3D%201.18-%23007d9c" alt="go>=1.18"></a>
  <a href="https:https://github.com/sendya/ant-design-pro-vue"><img src="https://img.shields.io/badge/UI-Ant%20Design%20Pro%20Vue-brightgreen" alt="UI"></a>
</p>

**`OneTerm`** 堡垒机，基于4A理念，即认证(Authen)、授权(Authorize)、账号(Account)、审计(Audit)设计开发。

`主要用途`：主要用于企业通过实现对IT人员操作行为的控制和审计来提升IT内部控制、合规安全性的产品。

`主要功能`：角色管理、授权审批、资源访问控制、会话审计等。

---
[English](README.md) / [中文](README_cn.md)
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
docker-compose up -d
```

## ✅验证
- 浏览器打开: [http://127.0.0.1:8000](http://127.0.0.1:8000)
- username: admin
- password: 123456

## SSH终端
### 效果
![Example GIF](./docs/images/ssh-client.gif)
### 登录
```shell
ssh -p12229 admin@127.0.0.1 # 注意这里端口,用户，地址需要换成您当前环境的
```
### 免密登录配置
> 终端免密登录是为了增加安全性以及便捷性而设计
1. 生成并获取公钥, 获取mac地址
```shell
ssh-keygen -t ed25519 # 根据提示生成key
cat /root/.ssh/id_ed25519.pub # 拷贝公钥, 公钥地址从上一步生成的过程中获取，如下图所示
ifconfig | grep -B1 "xxx.xxx.xxx.xxx" | awk '/ether/{print $2}' # 获取mac地址， 其中xxx.xxx.xxx.xxx换成您本机的IP
```

![img.png](docs/images/img.png)

2. 将公钥和mac放在平台上
![img_1.png](docs/images/img_1.png)

### 更精简的的登录方式
```shell
ssh oneterm
```
> 要达到这种效果，可进行如下配置
1. 创建ssh config文件
```shell
touch ~/.ssh/config
```
2. 将以下内容添加到 **`~/.ssh/config`**
```shell
Host oneterm
    HostName 127.0.0.1 # 此处替换为您oneterm的ssh server的地址
    Port 12229 # 此处替换为您oneterm的ssh server的端口
    User admin # 此处替换为您oneterm上的平台用户
```

## VNC/RDP
![rdp.png](docs/images/rdp.png)

## 🎯下一步计划
- [ ] 资产列表关联CMDB授权服务树
- [ ] web终端页面多个session支持tab等


## 📚产品文档

文档地址：https://veops.cn/docs/docs/oneterm/onterm_design

## 🎯计划

- [x] RDP
- [x] VNC
- [ ] 移除CTE以兼容mysql5.7

## 🔗相关项目

[go-ansiterm](https://github.com/veops/go-ansiterm)：linux终端仿真器,主要是根据终端输入和服务器回显解析命令

## 🤝社区交流

**欢迎关注公众号(维易科技OneOps)，关注后可加入微信群，进行产品和技术交流。**

![公众号: 维易科技OneOps](docs/images/wechat.jpg)