<template>
  <div>
    <ops-table
      ref="xTable"
      size="mini"
      stripe
      class="ops-stripe-table"
      :data="tableData"
      max-height="400px"
      show-overflow
      show-header-overflow
      resizable
      :row-config="{ keyField: 'rid' }"
      :scroll-y="{ enabled: true, gt: 30 }"
    >
      <vxe-column width="150px" field="name"></vxe-column>
      <vxe-column
        v-for="col in columns"
        :key="col.field"
        :field="col.field"
        :title="col.titleKey ? $t(col.titleKey) : col.label"
        width="148px"
      >
        <template #default="{row}">
          <a-checkbox
            v-model="row.permissions[col.field]"
            :disabled="pending[`${row.rid}:${col.field}`]"
            @change="(e) => handleChange(e, col, row)"
          ></a-checkbox>
        </template>
      </vxe-column>
    </ops-table>
  </div>
</template>

<script>
import {
  setRoleResourcePerm,
  deleteRoleResourcePerm
} from '@/modules/acl/api/permission'

export default {
  name: 'ACLTable',
  props: {
    tableData: {
      type: Array,
      default: () => []
    },
    resourceId: {
      type: [String, Number],
      default: ''
    },
    columns: {
      type: Array,
      default: () => []
    }
  },
  data() {
    return {
      pending: {}
    }
  },
  methods: {
    async handleChange(e, col, row) {
      const checked = e.target.checked
      const key = `${row.rid}:${col.field}`
      this.$set(this.pending, key, true)
      try {
        const update = checked ? setRoleResourcePerm : deleteRoleResourcePerm
        await update(row.rid, this.resourceId, {
          perms: [col.field],
          app_id: 'oneterm'
        })
        this.$message.success(this.$t('operateSuccess'))
      } catch (error) {
        this.$set(row.permissions, col.field, !checked)
      } finally {
        this.$delete(this.pending, key)
      }
    }
  }
}
</script>

<style lang="less" scoped>
</style>
