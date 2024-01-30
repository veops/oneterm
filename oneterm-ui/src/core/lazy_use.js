import Vue from 'vue'
import VueStorage from 'vue-ls'
import config from '@/config/setting'

// base library
import '@/core/lazy_lib/components_use'
import Viser from 'viser-vue'

// ext library
import VueClipboard from 'vue-clipboard2'
import PermissionHelper from '@/utils/helper/permission'
import './directives/action'

VueClipboard.config.autoSetContainer = true

Vue.use(Viser)

Vue.use(VueStorage, config.storageOptions)
Vue.use(VueClipboard)
Vue.use(PermissionHelper)
