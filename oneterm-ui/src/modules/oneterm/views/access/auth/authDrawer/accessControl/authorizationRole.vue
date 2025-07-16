<template>
  <a-form-model-item :label="$t('oneterm.auth.authorizationRole')" prop="rids">
    <EmployeeTreeSelect
      :value="rids"
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
      @change="handleRoleChange"
    />
  </a-form-model-item>
</template>

<script>
import { searchRole } from '@/modules/acl/api/role'

import EmployeeTreeSelect from '@/views/setting/components/employeeTreeSelect.vue'

export default {
  name: 'AuthorizationRole',
  components: {
    EmployeeTreeSelect
  },
  props: {
    form: {
      type: Object,
      default: () => {}
    }
  },
  data() {
    return {
      visualRoleList: []
    }
  },
  computed: {
    rids() {
      return (this.form?.rids || []).map((r) => `employee-${r}`)
    }
  },
  mounted() {
    this.getRoleList()
  },
  methods: {
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
    handleRoleChange(value) {
      const rids = (value || []).map((r) => Number(r.split('-')[1]))
      this.$emit('change', ['rids'], rids)
    }
  }
}
</script>
