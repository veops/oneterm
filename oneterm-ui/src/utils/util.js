import _ from 'lodash'
export function timeFix() {
  const time = new Date()
  const hour = time.getHours()
  return hour < 9 ? '早上好' : hour <= 11 ? '上午好' : hour <= 13 ? '中午好' : hour < 20 ? '下午好' : '晚上好'
}

export function welcome() {
  const arr = ['休息一会儿吧', '准备吃什么呢?', '要不要打一把 DOTA?', '我猜你可能累了', '你有一个小目标吗?']
  const index = Math.floor(Math.random() * arr.length)
  return arr[index]
}

/**
 * 触发 window.resize
 */
export function triggerWindowResizeEvent() {
  const event = document.createEvent('HTMLEvents')
  event.initEvent('resize', true, true)
  event.eventType = 'message'
  window.dispatchEvent(event)
}

export function handleScrollHeader(callback) {
  let timer = 0

  let beforeScrollTop = window.pageYOffset
  callback = callback || function () { }
  window.addEventListener(
    'scroll',
    event => {
      clearTimeout(timer)
      timer = setTimeout(() => {
        let direction = 'up'
        const afterScrollTop = window.pageYOffset
        const delta = afterScrollTop - beforeScrollTop
        if (delta === 0) {
          return false
        }
        direction = delta > 0 ? 'down' : 'up'
        callback(direction)
        beforeScrollTop = afterScrollTop
      }, 50)
    },
    false
  )
}

/**
 * Remove loading animate
 * @param id parent element id or class
 * @param timeout
 */
export function removeLoadingAnimate(id = '', timeout = 1500) {
  if (id === '') {
    return
  }
  setTimeout(() => {
    document.body.removeChild(document.getElementById(id))
  }, timeout)
}

export function debounce(fn, timeout = 300) {
  let timer
  return function (_this, ...args) {
    clearTimeout(timer)
    timer = setTimeout(() => {
      fn.apply(_this, args)
    }, timeout)
  }
}

export function copyArray(arr) {
  return arr.map((e) => {
    if (typeof e === 'object') {
      return Object.assign({}, e)
    } else {
      return e
    }
  })
}

export async function sleep(n) {
  return new Promise((resolve) => setTimeout(resolve, n))
}

export async function flicker(ele, property, value, interval = 300, times = 4) {
  const origin = ele.style[property]
  for (let i = 0; i < times * 2; i++) {
    if (i % 2) {
      ele.style[property] = value
    } else {
      ele.style[property] = origin
    }
    await sleep(interval)
  }
  ele.style[property] = origin
}

// 数字加逗号
export const toThousands = (num = 0) => {
  return num.toString().replace(/\d+/, function (n) {
    return n.replace(/(\d)(?=(?:\d{3})+$)/g, '$1,')
  })
}

// 从id得到部门名称
export const getDepartmentName = (allFlatDepartments, id) => {
  const _find = allFlatDepartments.find((item) => item.department_id === id)
  return _find?.department_name || ''
}

// 从id得到员工姓名
export const getDirectorName = (allFlatEmployees, id) => {
  const _find = allFlatEmployees.find((item) => item.employee_id === id)
  return _find?.nickname || ''
}

export const isEmptySubDepartments = (item) => {
  if (item.employees.length) {
    return false
  }
  for (let i = 0; i < item.sub_departments.length; i++) {
    if (!isEmptySubDepartments(item.sub_departments[i])) {
      return false
    }
  }
  return true
}

// format部门员工树
export const formatOption = (data, idType = 1, isDisabledAllCompany, departmentKey = 'department_id', employeeKey = 'employee_id') => {
  // idType  1  表示  员工id为`${department_id}-${employee_id}`
  //         2  表示  department-${department_id}   employee-${employee_id}
  let _data = _.cloneDeep(data)
  _data = _data.filter((item) => {
    return item.employees.length || (item.sub_departments.length && !isEmptySubDepartments(item))
  })
  _data.forEach((item) => {
    if (isDisabledAllCompany) {
      item.isDisabled = !item.department_id
    }
    item.id = idType === 1 ? item[departmentKey] : `department-${item[departmentKey]}`
    item.label = item.department_name
    item.children = [
      ...formatOption(
        item.sub_departments.map((dep) => {
          return { ...dep, id: idType === 1 ? dep[departmentKey] : `department-${dep[departmentKey]}`, label: dep.department_name }
        }),
        idType,
        isDisabledAllCompany,
        departmentKey,
        employeeKey
      ),
      ...item.employees.map((employee) => {
        return { ...employee, id: idType === 1 ? `${item[departmentKey]}-${employee[employeeKey]}` : `employee-${employee[employeeKey]}`, label: employee.nickname }
      }),
    ]
  })
  return _data
}

const msPerMinute = 60
const msPerHour = msPerMinute * 60
const msPerDay = msPerHour * 24
const msPerWeek = msPerDay * 7
const msPerMonth = msPerDay * 30
const msPerYear = msPerDay * 365

export function getRelativeTime(originElapsed, unit = 'sec', lang = 'en') {
  let elapsed = originElapsed
  if (unit === 'msec') {
    elapsed = originElapsed / 1000
  }
  if (elapsed < msPerMinute) {
    return `${Math.round(elapsed)}${lang === 'en' ? 's' : '秒'}`
  } else if (elapsed < msPerHour) {
    return `${(elapsed / msPerMinute).toFixed(1)}${lang === 'en' ? 'mins' : '分'}`
  } else if (elapsed < msPerDay) {
    return `${(elapsed / msPerHour).toFixed(1)}${lang === 'en' ? 'hours' : '时'}`
  } else if (elapsed < msPerWeek) {
    return `${(elapsed / msPerDay).toFixed(1)}${lang === 'en' ? 'days' : '天'}`
  } else if (elapsed < msPerMonth) {
    return `${(elapsed / msPerWeek).toFixed(1)}${lang === 'en' ? 'weeks' : '周'}`
  } else if (elapsed < msPerYear) {
    return `${(elapsed / msPerMonth).toFixed(1)}${lang === 'en' ? 'months' : '月'}`
  } else {
    return `${(elapsed / msPerYear).toFixed(1)}${lang === 'en' ? 'years' : '年'}`
  }
}
