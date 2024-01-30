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
