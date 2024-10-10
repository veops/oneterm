<template>
  <ops-table
    size="small"
    ref="opsTable"
    :data="tableData"
    :height="400"
    :width="600"
  >
    <vxe-column :title="$t(`oneterm.protocol`)" field="protocol" :width="80" show-overflow ></vxe-column>
    <vxe-column :title="$t(`oneterm.account`)" field="accountName" :width="100" show-overflow ></vxe-column>
    <vxe-column :title="$t(`oneterm.assetList.validTime`)" field="dateRange" :width="150" show-overflow ></vxe-column>
    <vxe-column :title="$t(`oneterm.assetList.times`)" field="timesStr" :width="80" show-overflow ></vxe-column>
    <vxe-column :title="$t(`oneterm.assetList.link`)" field="url">
      <template #default="{row}">
        <div class="temp-link-url">
          <a-tooltip :title="row.url">
            <span class="temp-link-url-text" >{{ row.url }}</span>
          </a-tooltip>
          <a class="temp-link-url-copy">
            <ops-icon
              type="veops-copy"
              @click="copyUrl(row.url)"
            />
          </a>
        </div>
      </template>
    </vxe-column>
  </ops-table>
</template>

<script>
import moment from 'moment'
import { getShareLink } from '@/modules/oneterm/api/connect.js'

export default {
  name: 'TempLinkTable',
  props: {
    assetData: {
      type: Object,
      default: () => {}
    },
    accountList: {
      type: Array,
      default: () => []
    }
  },
  data() {
    return {
      tableData: []
    }
  },
  mounted() {
    this.getTableData()
  },
  methods: {
    async getTableData() {
      const res = await getShareLink({
        page_index: 1,
        page_size: 9999,
        asset_id: this.assetData.id
      })

      const tableData = res?.data?.list || []
      tableData.forEach((item) => {
        const protocol = item?.protocol?.split(':')?.[0] || ''
        item.url = `${document.location.origin}/oneterm/share/${protocol}/${item.uuid}`

        const _find = this.accountList?.find((account) => Number(account.id) === Number(item.account_id))
        item.accountName = _find?.name ?? item.account_id

        item.dateRange = `${moment(item.start).format('YYYY-MM-DD HH:mm:ss')} - ${moment(item.end).format('YYYY-MM-DD HH:mm:ss')}`
        item.timesStr = item.no_limit ? this.$t('oneterm.assetList.any') : item.times
      })
      this.tableData = tableData
    },

    copyUrl(url) {
      this.$copyText(url)
        .then(() => {
          this.$message.success(this.$t('copySuccess'))
        })
    },
    refreshTable() {
      this.$nextTick(() => {
        this.$refs.opsTable.getVxetableRef().refreshScroll()
      })
    }
  }
}
</script>

<style lang="less" scoped>
.temp-link-url {
  display: flex;
  align-items: center;

  &-text {
    max-width: 100%;
    text-overflow: ellipsis;
    overflow: hidden;
    text-wrap: nowrap;
  }

  &-copy {
    flex-shrink: 0;
    margin-left: 12px;
  }
}
</style>
