<template>
  <a-row class="form-account">
    <a-col v-bind="colSpan">
      <vxe-table
        ref="xTable"
        size="mini"
        :data="authList"
        :column-config="{ width: 200 }"
        :min-height="80"
      >
        <vxe-column
          v-if="!hasWebProtocol"
          field="account"
          width="190"
        >
          <template #header>
            <div class="table-account-header" slot="header">
              <span>{{ $t('oneterm.account') }}</span>
              <a-tooltip :title="$t('oneterm.assetList.accountFormItemTip')">
                <a-icon type="question-circle" />
              </a-tooltip>
            </div>
          </template>
          <template #default="{ row }">
            <a-select
              v-model="row.account"
              showSearch
              :style="{
                width: '180px',
              }"
              :placeholder="$t('placeholder2')"
              optionFilterProp="title"
              allowClear
            >
              <a-select-option
                v-for="(node, nodeIndex) in accountList"
                :key="node.id + nodeIndex"
                :value="node.id"
                :title="node.name"
              >
                <a-tooltip :title="node.toolTip">
                  {{ node.name }}
                  <span v-if="node.account" class="select-option-name">({{ node.account }})</span>
                </a-tooltip>
              </a-select-option>
            </a-select>
          </template>
        </vxe-column>
        <vxe-column
          field="grantUser"
          :title="$t('oneterm.assetList.grantRole')"
          :width="hasWebProtocol ? 'auto' : '190'"
        >
          <template #default="{ row }">
            <EmployeeTreeSelect
              v-model="row.rids"
              multiple
              :idType="2"
              departmentKey="acl_rid"
              employeeKey="acl_rid"
              :placeholder="`${$t(`placeholder2`)}`"
              class="custom-treeselect custom-treeselect-white"
              :style="{
                '--custom-height': '32px',
                lineHeight: '32px',
                '--custom-multiple-lineHeight': '18px',
              }"
              :limit="1"
              :otherOptions="visualRoleList"
            />
          </template>
        </vxe-column>
        <vxe-column
          field="permissions"
          :title="$t('oneterm.assetList.operationPermissions')"
          :width="hasWebProtocol ? 'auto' : '230'"
        >
          <template #default="{ row }">
            <PermissionCheckbox
              :value="row.permissions"
              :hideOptions="hideOperationPermissionOptions"
              @change="(key, checked) => row.permissions[key] = checked"
            />
          </template>
        </vxe-column>
        <vxe-column field="operation" :title="$t('operation')" width="55" fixed="right">
          <template #default="{ row }">
            <a-space>
              <a @click="addCount"><a-icon type="plus-circle"/></a>
              <a v-if="authList && authList.length > 1" @click="deleteCount(row.id)"><a-icon type="minus-circle"/></a>
            </a-space>
          </template>
        </vxe-column>
      </vxe-table>
    </a-col>
  </a-row>
</template>

<script>
import _ from 'lodash'
import { v4 as uuidv4 } from 'uuid'
import { getAccountList } from '@/modules/oneterm/api/account'
import { searchRole } from '@/modules/acl/api/role'
import { getConfig } from '@/modules/oneterm/api/config'
import { PERMISSION_TYPE } from '@/modules/oneterm/views/systemSettings/accessControl/constants.js'

import EmployeeTreeSelect from '@/views/setting/components/employeeTreeSelect.vue'
import PermissionCheckbox from '@/modules/oneterm/views/systemSettings/accessControl/permissionCheckbox.vue'

const DEFAULT_PERMISSIONS = Object.values(PERMISSION_TYPE).reduce((config, key) => {
  config[key] = false
  return config
}, {})

export default {
  name: 'Account',
  components: {
    EmployeeTreeSelect,
    PermissionCheckbox
  },
  props: {
    colSpan: {
      type: Object,
      default: () => ({
        span: 17,
        offset: 4,
      }),
    },
    resourceType: {
      type: String,
      default: 'asset'
    },
    protocolTypeList: {
      type: Array,
      default: () => []
    }
  },
  data() {
    return {
      accountList: [],
      authList: [{
        id: uuidv4(),
        account: undefined,
        rids: undefined,
        permissions: { ...DEFAULT_PERMISSIONS }
      }],
      visualRoleList: [],
      hasWebProtocol: false,
      hideOperationPermissionOptions: []
    }
  },
  watch: {
    protocolTypeList: {
      immediate: true,
      deep: true,
      handler(protocolTypeList) {
        this.handleWatchProtocolTypeList(protocolTypeList)
      }
    }
  },
  created() {
    this.initDefaultPermissions()
    this.getRoleList()
    this.getAccountList()
  },
  methods: {
    initDefaultPermissions() {
      getConfig({
        info: true
      }).then((res) => {
        const default_permissions = res?.data?.default_permissions
        Object.keys(DEFAULT_PERMISSIONS).forEach((key) => {
          DEFAULT_PERMISSIONS[key] = default_permissions?.[key] ?? DEFAULT_PERMISSIONS[key]
        })
        this.authList.forEach((item) => {
          if (!item.rule_id) {
            item.permissions = { ...DEFAULT_PERMISSIONS }
          }
        })
      })
    },

    async getRoleList() {
      const res = await searchRole({ page_size: 9999, page: 1, app_id: 'oneterm', user_role: 0, user_only: 0, is_all: true })

      const visualRoleList = []
      const roleList = (res?.roles || []).filter((item) => !/_virtual$/.test(item.name))

      if (roleList.length) {
        visualRoleList.push({
          acl_rid: -100,
          department_name: this.$t('acl.visualRole'),
          sub_departments: [],
          employees: roleList.map((item) => {
            return {
              nickname: item.name,
              acl_rid: item.id
            }
          })
        })
      }

      this.$set(this, 'visualRoleList', visualRoleList)
    },

    async getAccountList() {
      const res = await getAccountList({ page_index: 1 })
      const accountList = res?.data?.list || []
      accountList.forEach((item) => {
        item.toolTip = item.name + (item.account ? '(item.account)' : '')
      })
      this.accountList = accountList
    },

    handleWatchProtocolTypeList(protocolTypeList) {
      const hasRDP = protocolTypeList.includes('rdp')
      const hasSSH = protocolTypeList.includes('ssh')
      const hasHTTP = protocolTypeList.includes('http')
      const hasHTTPS = protocolTypeList.includes('https')
      const hasWebProtocol = hasHTTP || hasHTTPS

      const hideOptions = []

      if (this.resourceType === 'asset') {
        if (!hasRDP) {
          hideOptions.push(PERMISSION_TYPE.COPY, PERMISSION_TYPE.PASTE)
        }
        if (!(hasSSH || hasRDP || hasHTTP || hasHTTPS)) {
          hideOptions.push(PERMISSION_TYPE.FILE_DOWNLOAD)
        }
        if (!(hasSSH || hasRDP)) {
          hideOptions.push(PERMISSION_TYPE.FILE_UPLOAD)
        }
        if (hasWebProtocol) {
          hideOptions.push(PERMISSION_TYPE.SHARE)
        }
      }
      this.hideOperationPermissionOptions = hideOptions

      let authList = this.authList
      if (hasWebProtocol) {
        const auth = this.authList.slice(0, 1)
        authList = auth
      } else {
        authList.forEach((auth) => {
          if (auth.account === -1) {
            auth.account = undefined
          }
        })
      }
      this.authList = authList
      this.hasWebProtocol = hasWebProtocol
    },

    addCount() {
      this.authList.push({
        id: uuidv4(),
        account: undefined,
        rids: undefined,
        permissions: { ...DEFAULT_PERMISSIONS }
      })
    },
    deleteCount(id) {
      const index = this.authList.findIndex((item) => item.id === id)
      if (index !== -1) {
        this.authList.splice(index, 1)
      }
    },

    getValues() {
      const authorization = {}

      const authList = _.cloneDeep(this.authList)
      if (this.hasWebProtocol) {
        authList.forEach((auth) => {
          auth.account = -1
        })
      }

      authList
        .filter((auth) => auth.account)
        .forEach((auth) => {
          const rids = (auth?.rids || []).map((r) => Number(r.split('-')[1]))
          const authorizationItem = {
            rids,
            permissions: auth.permissions
          }
          if (auth?.rule_id) {
            authorizationItem.rule_id = auth.rule_id
          }
          authorization[auth.account] = authorizationItem
        })
      return { authorization }
    },

    setValues({ authorization = {} }) {
      const authorizationList = Object.entries(authorization || {})
      if (authorizationList.length) {
        this.authList = authorizationList.map(([key, value]) => {
          return {
            id: uuidv4(),
            account: Number(key),
            rule_id: value?.rule_id ?? undefined,
            rids: (value?.rids || []).map((r) => `employee-${r}`),
            permissions: value.permissions
          }
        })
      } else {
        this.authList = [{
          id: uuidv4(),
          account: undefined,
          rids: undefined,
          permissions: { ...DEFAULT_PERMISSIONS }
        }]
      }
    },
  },
}
</script>

<style lang="less" scoped>
.form-account {
  /deep/ .ant-checkbox-wrapper {
    margin-right: 0px;
    width: 48%;
  }
}

.table-account-header {
  display: inline-flex;
  align-items: center;

  i {
    margin-left: 4px;
    color: #4e5969;
    cursor: pointer;
  }
}

.select-option-name {
  font-size: 12px;
  color: #A5A9BC;
}
</style>
