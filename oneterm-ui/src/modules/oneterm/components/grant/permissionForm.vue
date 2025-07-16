<template>
  <a-form-model
    ref="permissionFormRef"
    :model="form"
    :rules="rules"
    :label-col="{ span: 5 }"
    :wrapper-col="{ span: 16 }"
  >
    <a-form-model-item :label="$t('oneterm.assetList.grantRole')" prop="rids">
      <EmployeeTreeSelect
        v-model="form.rids"
        multiple
        :idType="2"
        departmentKey="acl_rid"
        employeeKey="acl_rid"
        :placeholder="$t('placeholder2')"
        class="custom-treeselect custom-treeselect-white"
        :style="{
          '--custom-height': '32px',
          lineHeight: '32px',
          '--custom-multiple-lineHeight': '18px',
        }"
        :limit="1"
        :otherOptions="visualRoleList"
      />
    </a-form-model-item>
    <a-form-model-item :label="$t('oneterm.assetList.operationPermissions')" prop="permissions">
      <PermissionCheckbox
        :value="form.permissions"
        @change="(key, checked) => form.permissions[key] = checked"
      />
    </a-form-model-item>
    <div class="permission-form-operation">
      <a-button @click="handleCancel">
        {{ $t('cancel') }}
      </a-button>

      <a-button
        type="primary"
        @click="handleOk"
      >
        {{ $t('confirm') }}
      </a-button>
    </div>
  </a-form-model>
</template>

<script>
import { v4 as uuidv4 } from 'uuid'
import { postAuth } from '@/modules/oneterm/api/authorizationV2.js'
import { searchRole } from '@/modules/acl/api/role'
import { getConfig } from '@/modules/oneterm/api/config'
import { PERMISSION_TYPE } from '@/modules/oneterm/views/systemSettings/accessControl/constants.js'
import { TARGET_SELECT_TYPE } from '@/modules/oneterm/views/access/auth/constants.js'

import EmployeeTreeSelect from '@/views/setting/components/employeeTreeSelect.vue'
import PermissionCheckbox from '@/modules/oneterm/views/systemSettings/accessControl/permissionCheckbox.vue'

const DEFAULT_PERMISSIONS = Object.values(PERMISSION_TYPE).reduce((config, key) => {
  config[key] = false
  return config
}, {})

export default {
  name: 'PermissionForm',
  components: {
    EmployeeTreeSelect,
    PermissionCheckbox
  },
  props: {
    dataType: {
      type: String,
      default: 'node'
    },
    ids: {
      type: Array,
      default: () => []
    }
  },
  data() {
    return {
      form: {
        rids: [],
        permissions: { ...DEFAULT_PERMISSIONS }
      },
      rules: {
        rids: [{ required: true, message: this.$t('placeholder2') }],
      },
      visualRoleList: []
    }
  },
  mounted() {
    this.initDefaultPermissions()
    this.loadRoles()
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
        this.form.permissions = { ...DEFAULT_PERMISSIONS }
      })
    },
    async loadRoles() {
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

      this.visualRoleList = visualRoleList
    },
    handleCancel() {
      this.resetFields()
      this.$emit('cancel')
    },
    resetFields() {
      this.form = {
        rids: [],
        permissions: { ...DEFAULT_PERMISSIONS }
      }
      this.$refs.permissionFormRef.resetFields()
    },
    handleOk() {
      this.$refs.permissionFormRef.validate(async (valid) => {
        if (!valid) return
        const params = this.handleParams()
        postAuth(params).then(() => {
          this.$message.success(this.$t('updateSuccess'))
          this.handleCancel()
        })
      })
    },
    handleParams() {
      const rids = this.form.rids.map((rid) => {
        return Number(rid?.split?.('-')?.[1])
      })
      const params = {
        name: `${this.dataType}-${this.ids.join(',')}-${uuidv4()}`,
        rids,
        permissions: this.form.permissions
      }

      ;['node', 'account', 'asset'].forEach((key) => {
        params[`${key}_selector`] = {
          type: TARGET_SELECT_TYPE.ID,
          values: key === this.dataType ? this.ids.map((id) => String(id)) : []
        }
      })

      return params
    },
  }
}
</script>

<style lang="less" scoped>
.permission-form-operation {
  display: flex;
  justify-content: flex-end;
  column-gap: 8px;
}
</style>
