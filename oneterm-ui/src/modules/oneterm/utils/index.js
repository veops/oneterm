export const getAllParentNodesLabel = (node, label) => {
    if (node.parentNode) {
        return getAllParentNodesLabel(node.parentNode, `${node.parentNode.label}-${label}`)
    }
    return label
}
export const getTreeSelectLabel = (node) => {
    return `${getAllParentNodesLabel(node, node.label)}`
}

export const setLocalStorage = (name, param) => {
    let storageData = JSON.parse(localStorage.getItem(name))
    if (storageData) {
        storageData = { ...storageData, ...param }
    } else {
        storageData = { ...param }
    }
    localStorage.setItem(name, JSON.stringify(storageData))
}

class Strings {
    hasText = function (text) {
        return !(text === undefined || text === null || text.length === 0)
    }

    zeroPad = function zeroPad(num, minLength) {
        let str = num.toString()
        while (str.length < minLength) { str = '0' + str }
        return str
    };
}

export const strings = new Strings()

class Times {
    formatTime = function formatTime(millis) {
        const totalSeconds = Math.floor(millis / 1000)

        // Split into seconds and minutes
        const seconds = totalSeconds % 60
        const minutes = Math.floor(totalSeconds / 60)

        // Format seconds and minutes as MM:SS
        return strings.zeroPad(minutes, 2) + ':' + strings.zeroPad(seconds, 2)
    };
}

export const times = new Times()
