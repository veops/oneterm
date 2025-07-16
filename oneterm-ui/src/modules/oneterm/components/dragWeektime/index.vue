<template>
  <div class="c-weektime">
    <div class="c-schedue"></div>
    <div :class="{ 'c-schedue': true, 'c-schedue-notransi': mode }" :style="styleValue"></div>

    <table :class="{ 'c-min-table': colspan < 2 }" class="c-weektime-table">
      <thead class="c-weektime-head">
        <tr>
          <th rowspan="8" class="week-td">{{ $t('oneterm.timeTemplate.weektime') }}</th>
          <th :colspan="12 * colspan">00:00 - 12:00</th>
          <th :colspan="12 * colspan">12:00 - 24:00</th>
        </tr>
        <tr>
          <td v-for="t in hourArr" :key="t" :colspan="colspan">{{ t }}</td>
        </tr>
      </thead>
      <tbody class="c-weektime-body">
        <tr v-for="t in data" :key="t.row">
          <td>{{ $t(`oneterm.timeTemplate.day${t.day}`) }}</td>
          <td
            v-for="n in t.child"
            :key="`${n.row}-${n.col}`"
            :data-week="n.row"
            :data-time="n.col"
            :class="selectClasses(n)"
            @mouseenter="cellEnter(n)"
            @mousedown="cellDown(n)"
            @mouseup="cellUp(n)"
            class="weektime-atom-item"
          ></td>
        </tr>
        <tr>
          <td colspan="49" class="c-weektime-preview">
            <div class="g-clearfix c-weektime-con">
              <span class="g-pull-left">
                {{ selectState ? $t('oneterm.timeTemplate.selectedTime') : $t('oneterm.timeTemplate.drag') }}
              </span>
              <a @click.prevent="$emit('onClear')" class="g-pull-right">{{ $t(`clear`) }}</a>
            </div>
            <div v-if="selectState" class="c-weektime-time">
              <div v-for="t in selectValue" :key="t.id">
                <p v-if="t.value && t.value.length">
                  <span class="g-tip-text">{{ $t(`oneterm.timeTemplate.day${t.day}`) }}：</span>
                  <span>{{ formatSelectValue(t.value) }}</span>
                </p>
              </div>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script>
const createHour = (len) => {
  return Array.from(Array(len)).map((ret, id) => id)
}
export default {
  name: 'DragWeektime',
  model: {
    prop: 'value',
    event: 'change',
  },
  props: {
    value: {
      type: Array,
      required: true,
    },
    data: {
      type: Array,
      required: true,
    },
    colspan: {
      type: Number,
      default() {
        return 2
      },
    },
  },
  data() {
    return {
      width: 0,
      height: 0,
      left: 0,
      top: 0,
      mode: 0,
      row: 0,
      col: 0,
      hourArr: [],
    }
  },
  computed: {
    styleValue() {
      return {
        width: `${this.width}px`,
        height: `${this.height}px`,
        left: `${this.left}px`,
        top: `${this.top}px`,
      }
    },
    selectValue() {
      return this.value
    },
    selectState() {
      return this.value.some((ret) => ret.value && ret.value.length)
    },
    selectClasses() {
      return (n) => (n.check ? 'ui-selected' : '')
    },
  },
  created() {
    this.hourArr = createHour(24)
  },
  methods: {
    cellEnter(item) {
      const ele = document.querySelector(`td[data-week='${item.row}'][data-time='${item.col}']`)
      if (ele && !this.mode) {
        this.left = ele.offsetLeft
        this.top = ele.offsetTop
      } else if (item.col <= this.col && item.row <= this.row) {
        this.width = (this.col - item.col + 1) * ele.offsetWidth
        this.height = (this.row - item.row + 1) * ele.offsetHeight
        this.left = ele.offsetLeft
        this.top = ele.offsetTop
      } else if (item.col >= this.col && item.row >= this.row) {
        this.width = (item.col - this.col + 1) * ele.offsetWidth
        this.height = (item.row - this.row + 1) * ele.offsetHeight
        if (item.col > this.col && item.row === this.row) this.top = ele.offsetTop
        if (item.col === this.col && item.row > this.row) this.left = ele.offsetLeft
      } else if (item.col > this.col && item.row < this.row) {
        this.width = (item.col - this.col + 1) * ele.offsetWidth
        this.height = (this.row - item.row + 1) * ele.offsetHeight
        this.top = ele.offsetTop
      } else if (item.col < this.col && item.row > this.row) {
        this.width = (this.col - item.col + 1) * ele.offsetWidth
        this.height = (item.row - this.row + 1) * ele.offsetHeight
        this.left = ele.offsetLeft
      }
    },
    cellDown(item) {
      const ele = document.querySelector(`td[data-week='${item.row}'][data-time='${item.col}']`)
      this.check = Boolean(item.check)
      this.mode = 1
      if (ele) {
        this.width = ele.offsetWidth
        this.height = ele.offsetHeight
      }

      this.row = item.row
      this.col = item.col
    },
    cellUp(item) {
      if (item.col <= this.col && item.row <= this.row) {
        this.selectWeek([item.row, this.row], [item.col, this.col], !this.check)
      } else if (item.col >= this.col && item.row >= this.row) {
        this.selectWeek([this.row, item.row], [this.col, item.col], !this.check)
      } else if (item.col > this.col && item.row < this.row) {
        this.selectWeek([item.row, this.row], [this.col, item.col], !this.check)
      } else if (item.col < this.col && item.row > this.row) {
        this.selectWeek([this.row, item.row], [item.col, this.col], !this.check)
      }

      this.width = 0
      this.height = 0
      this.mode = 0
    },
    selectWeek(row, col, check) {
      const [minRow, maxRow] = row
      const [minCol, maxCol] = col
      this.data.forEach((item) => {
        item.child.forEach((t) => {
          if (t.row >= minRow && t.row <= maxRow && t.col >= minCol && t.col <= maxCol) {
            this.$set(t, 'check', check)
          }
        })
      })
      const _selectValue = this.data.map((item) => {
        return {
          id: item.row,
          day: item.day,
          value: item.child.filter((c) => c.check).map((c) => c.value),
        }
      })
      this.$emit('change', _selectValue)
    },
    formatSelectValue(list) {
      const _list = []
      if (list && list.length) {
        const splitTime = list[0].split('~')
        let start = splitTime[0]
        let end = splitTime[1]
        list.forEach((time, index) => {
          if (index < list.length - 1) {
            const _splitTime = list[index + 1].split('~')
            const _start = _splitTime[0]
            const _end = _splitTime[1]
            if (end === _start) {
              end = _end
            } else {
              _list.push(`${start}~${end}`)
              start = _start
              end = _end
            }
          } else {
            _list.push(`${start}~${end}`)
          }
        })
      }
      return _list.join('，')
    },
  },
}
</script>

<style lang="less" scoped>
.c-weektime {
  min-width: 640px;
  position: relative;
  display: inline-block;
}
.c-schedue {
  background: #598fe6;
  position: absolute;
  width: 0;
  height: 0;
  opacity: 0.6;
  pointer-events: none;
}
.c-schedue-notransi {
  transition: width 0.12s ease, height 0.12s ease, top 0.12s ease, left 0.12s ease;
}
.c-weektime-table {
  border-collapse: collapse;
  th {
    vertical-align: inherit;
    font-weight: bold;
  }
  tr {
    height: 30px;
  }
  tr,
  td,
  th {
    user-select: none;
    border: 1px solid #dee4f5;
    text-align: center;
    min-width: 12px;
    line-height: 1.8em;
    transition: background 0.2s ease;
  }
  .c-weektime-head {
    font-size: 12px;
    .week-td {
      width: 70px;
    }
  }
  .c-weektime-body {
    font-size: 12px;
    td {
      &.weektime-atom-item {
        user-select: unset;
        background-color: #f5f5f5;
      }
      &.ui-selected {
        background-color: #598fe6;
      }
    }
  }
  .c-weektime-preview {
    line-height: 2.4em;
    padding: 0 10px;
    font-size: 14px;
    .c-weektime-con {
      line-height: 46px;
      user-select: none;
    }
    .c-weektime-time {
      text-align: left;
      line-height: 2.4em;
      p {
        max-width: 625px;
        line-height: 1.4em;
        word-break: break-all;
        margin-bottom: 8px;
      }
    }
  }
}
.c-min-table {
  tr,
  td,
  th {
    min-width: 24px;
  }
}
.g-clearfix {
  &:after,
  &:before {
    clear: both;
    content: ' ';
    display: table;
  }
}
.g-pull-left {
  float: left;
}
.g-pull-right {
  float: right;
}
.g-tip-text {
  color: #999;
}
</style>
