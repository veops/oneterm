<template>
  <div class="public-key">
    <a-spin :tip="loadTip" :spinning="loading">
      <div class="public-key-header">
        <a-space>
          <a-input-search
            :placeholder="$t('placeholderSearch')"
            @search="getPublicKey()"
            v-model="filterName"
            allow-clear
            class="ops-input ops-input-radius"
            style="width: 260px"
          >
          </a-input-search>

          <div class="ops-list-batch-action" v-if="!!selectedRowKeys.length">
            <span @click="batchDelete">{{ $t(`delete`) }}</span>
            <span>{{ $t('selectRows', { rows: selectedRowKeys.length }) }}</span>
          </div>

          <a-tooltip :title="$t('oneterm.systemSettings.publicKeyTip')">
            <a-icon style="font-size: 16px;" type="info-circle" />
          </a-tooltip>
        </a-space>
        <a-space>
          <a-button @click="create" type="primary">{{ $t(`create`) }}</a-button>
          <a-button @click="getPublicKey()">{{ $t(`refresh`) }}</a-button>
        </a-space>
      </div>
      <ops-table
        size="small"
        ref="opsTable"
        stripe
        class="ops-stripe-table"
        :data="tableData"
        show-overflow
        show-header-overflow
        @checkbox-change="onSelectChange"
        @checkbox-all="onSelectChange"
        @checkbox-range-end="onSelectRangeEnd"
        :checkbox-config="{ reserve: true, highlight: true, range: true }"
        :row-config="{ keyField: 'id' }"
        :height="tableHeight"
        resizable
      >
        <vxe-column type="checkbox" width="60px"></vxe-column>
        <vxe-column field="name" :title="$t(`oneterm.name`)"></vxe-column>
        <vxe-column field="pk" :title="$t(`oneterm.publicKey`)"></vxe-column>
        <!-- <vxe-column field="mac" :title="$t(`oneterm.macAddress`)"></vxe-column> -->
        <vxe-column field="created_at" :title="$t(`created_at`)" width="120">
          <template #default="{ row }">
            {{ moment(row.created_at).format('YYYY-MM-DD') }}
          </template>
        </vxe-column>
        <vxe-column field="operation" :title="$t(`operation`)" width="100">
          <template #default="{ row }">
            <a-space>
              <a @click="editPublicKey(row)"><ops-icon type="icon-xianxing-edit"/></a>
              <a-popconfirm :title="$t('confirmDelete')" @confirm="deletePublicKey(row)">
                <a style="color:red"><ops-icon type="icon-xianxing-delete"/></a>
              </a-popconfirm>
            </a-space>
          </template>
        </vxe-column>
      </ops-table>
      <div class="public-key-pagination">
        <a-pagination
          size="small"
          show-size-changer
          :current="currentPage"
          :total="totalResult"
          :show-total="
            (total, range) =>
              $t('pagination.total', {
                range0: range[0],
                range1: range[1],
                total,
              })
          "
          :page-size="pageSize"
          :default-current="1"
          @change="pageOrSizeChange"
          @showSizeChange="pageOrSizeChange"
        />
      </div>
    </a-spin>
    <editModal ref="editModal" @refresh="getPublicKey" />
  </div>
</template>

<script>
import { mapState } from 'vuex'
import moment from 'moment'
import { getPublicKeyList, deletePublicKeyById } from '@/modules/oneterm/api/publicKey.js'
import editModal from './editModal.vue'
export default {
  name: 'PublicKey',
  components: {
    editModal,
  },
  mounted() {
    this.getPublicKey()
  },
  data() {
    return {
      loadTip: '',
      loading: false,
      filterName: '',
      tableData: [],
      selectedRowKeys: [],
      currentPage: 1,
      pageSize: 20,
      totalResult: 0,
    }
  },
  computed: {
    ...mapState({
      windowHeight: (state) => state.windowHeight,
    }),
    tableHeight() {
      return this.windowHeight - 205
    },
  },
  methods: {
    moment,
    rangeTimeChange() {},
    editPublicKey(row) {
      this.$refs.editModal.open('edit', row)
    },
    deletePublicKey(row) {
      this.loading = true
      deletePublicKeyById(row.id)
        .then((res) => {
          this.$message.success(this.$t('deleteSuccess'))
          this.getPublicKey()
        })
        .finally(() => {
          this.loading = false
        })
    },
    onSelectChange() {
      const opsTable = this.$refs.opsTable.getVxetableRef()
      const records = [...opsTable.getCheckboxRecords(), ...opsTable.getCheckboxReserveRecords()]
      this.selectedRowKeys = records.map((i) => i.id)
    },
    onSelectRangeEnd({ records }) {
      this.selectedRowKeys = records.map((i) => i.ci_id || i._id)
    },
    async batchDelete() {
      const that = this
      this.$confirm({
        title: that.$t('warning'),
        content: that.$t('confirmDelete'),
        async onOk() {
          let successNum = 0
          let errorNum = 0
          this.loading = true
          this.loadTip = `${that.$t('deleting')}...`
          for (let i = 0; i < this.selectedRowKeys.length; i++) {
            await deletePublicKeyById(this.selectedRowKeys[i], false)
              .then(() => {
                successNum += 1
              })
              .catch(() => {
                errorNum += 1
              })
              .finally(() => {
                this.loadTip = that.$t('deletingTip', { total: that.selectedRowKeys.length, successNum, errorNum })
              })
          }
          this.loading = false
          this.loadTip = ''
          this.selectedRowKeys = []
          this.$refs.opsTable.getVxetableRef().clearCheckboxRow()
          this.$refs.opsTable.getVxetableRef().clearCheckboxReserve()
          this.$nextTick(() => {
            this.getPublicKey()
          })
        },
      })
    },
    pageOrSizeChange(currentPage, pageSize) {
      this.currentPage = currentPage
      this.pageSize = pageSize
      this.getPublicKey()
    },
    create() {
      this.$refs.editModal.open('add', {})
    },
    getPublicKey() {
      this.loading = true
      getPublicKeyList({
        page_index: this.currentPage,
        page_size: this.pageSize,
        name: this.filterName,
      })
        .then((res) => {
          this.tableData = res?.data?.list || []
          this.totalResult = res?.data?.count ?? 0
        })
        .finally(() => {
          this.loading = false
        })
    },
  },
}
</script>

<style lang="less" scoped>
.public-key {
  background-color: #fff;
  height: 100%;
  border-radius: 6px;
  padding: 18px;

  &-header {
    display: flex;
    justify-content: space-between;
    margin-bottom: 16px;
  }
  &-pagination {
    text-align: right;
    margin-top: 8px;
  }
}
</style>
