const appConfig = {
    buildModules: ['oneterm', 'acl'], // 需要编译的模块
    redirectTo: '/oneterm', // 首页的重定向路径
    buildAclToModules: true, // 是否在各个应用下 内联权限管理
    showDocs: false,
    useEncryption: false,
}

export default appConfig
