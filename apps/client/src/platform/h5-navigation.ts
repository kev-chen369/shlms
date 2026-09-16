// uni-app H5 renders tab items as clickable divs, not native buttons.
export function enhanceH5Navigation(root: HTMLElement): () => void {
  function enhance() {
    root.querySelectorAll<HTMLElement>('.uni-tabbar__item').forEach(item => {
      item.tabIndex = 0
      item.setAttribute('role', 'button')
      item.setAttribute('aria-label', item.querySelector('.uni-tabbar__label')?.textContent?.trim() || item.textContent?.trim() || '导航')
    })
  }
  function activate(event: KeyboardEvent) {
    if (event.repeat || (event.key !== 'Enter' && event.key !== ' ')) return
    const item = event.target instanceof Element ? event.target.closest<HTMLElement>('.uni-tabbar__item') : null
    if (!item || !root.contains(item)) return
    event.preventDefault()
    item.click()
  }
  enhance()
  const observer = new MutationObserver(enhance)
  observer.observe(root, { childList: true, subtree: true, characterData: true })
  root.addEventListener('keydown', activate)
  return () => {
    observer.disconnect()
    root.removeEventListener('keydown', activate)
  }
}
