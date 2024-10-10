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
    >
      <vxe-column width="150px" field="name"></vxe-column>
      <vxe-column
        v-for="col in columns"
        :key="col.field"
        :field="col.field"
        :title="$t(col.title)"
        width="148px"
      >
        <template #default="{row}">
          <a-checkbox
            v-model="row[col.field]"
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
    }
  },
  data() {
    return {
      columns: [
        {
          field: 'read',
          title: 'view'
        },
        {
          field: 'write',
          title: 'update'
        },
        {
          field: 'delete',
          title: 'delete'
        },
        {
          field: 'grant',
          title: 'grant'
        }
      ]
    }
  },
  methods: {
    handleChange(e, col, row) {
      const checked = e.target.checked
      if (checked) {
        setRoleResourcePerm(row.rid, this.resourceId, {
          perms: [col.field],
          app_id: 'oneterm'
        }).then(() => {
          this.$message.success(this.$t('operateSuccess'))
        })
      } else {
        deleteRoleResourcePerm(row.rid, this.resourceId, {
          perms: [col.field],
          app_id: 'oneterm'
        }).then(() => {
          this.$message.success(this.$t('deleteSuccess'))
        })
      }
    }
  }
}
</script>

<style lang="less" scoped>
</style>
