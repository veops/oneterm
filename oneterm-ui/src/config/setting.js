/**
 * Project default configuration
 * primaryColor - Default theme color. If color change does not take effect, please clear localStorage.
 * navTheme - Sidebar theme ['dark', 'light']
 * colorWeak - Color blindness mode
 * layout - Overall layout ['sidemenu', 'topmenu']
 * fixedHeader - Sticky Header : boolean
 * fixSiderbar - Sticky sidebar : boolean
 * autoHideHeader - Hide Header when scrolling down : boolean
 * contentWidth - Content area layout: Fluid | Fixed
 *
 * storageOptions: {} - Vue-ls plugin options (localStorage/sessionStorage)
 */

export default {
  primaryColor: '#2f54eb', // primary color of ant design
  navTheme: 'dark', // theme for nav menu
  layout: 'sidemenu', // nav menu position: sidemenu or topmenu
  contentWidth: 'Fixed', // layout of content: Fluid or Fixed, only works when layout is topmenu
  fixedHeader: true, // sticky header
  fixSiderbar: true, // sticky siderbar
  autoHideHeader: true, //  auto hide header
  colorWeak: false,
  multiTab: false,
  production: process.env.NODE_ENV === 'production' && process.env.VUE_APP_PREVIEW !== 'true',
  // vue-ls options
  storageOptions: {
    namespace: 'pro__', // key prefix
    name: 'ls', // name variable Vue.[ls] or this.[$ls],
    storage: 'local' // storage name session, local, memory
  }
}
