const appConfig = {
  buildModules: ['oneterm', 'acl'], // Modules to be compiled
  redirectTo: '/oneterm', // Redirect path for the home page
  buildAclToModules: true, // Inline permission management in each app
  showDocs: false,
  useEncryption: true,
}

export default appConfig
