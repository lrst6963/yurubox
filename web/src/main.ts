import { createApp } from 'vue'
import App from './App.vue'
import './style.css'

// 注入全局滚动条样式到所有 Shadow DOM
const scrollbarSheet = new CSSStyleSheet()
scrollbarSheet.replaceSync(`
  ::-webkit-scrollbar {
    width: 8px;
    height: 8px;
  }
  ::-webkit-scrollbar-track {
    background: transparent;
  }
  ::-webkit-scrollbar-thumb {
    background-color: var(--md-sys-color-outline-variant, #c3c7cf);
    border-radius: 4px;
    border: 2px solid transparent;
    background-clip: content-box;
  }
  ::-webkit-scrollbar-thumb:hover {
    background-color: var(--md-sys-color-outline, #73777f);
  }
  ::-webkit-scrollbar-corner {
    background: transparent;
  }
  * {
    scrollbar-width: thin;
    scrollbar-color: var(--md-sys-color-outline-variant, #c3c7cf) transparent;
  }
`)

const shadowRootProto = typeof ShadowRoot !== 'undefined' ? ShadowRoot.prototype : null
if (shadowRootProto) {
  const descriptor = Object.getOwnPropertyDescriptor(shadowRootProto, 'adoptedStyleSheets')
  if (descriptor && descriptor.set) {
    const originalSet = descriptor.set
    Object.defineProperty(shadowRootProto, 'adoptedStyleSheets', {
      set(sheets: CSSStyleSheet[]) {
        const filtered = sheets.filter(s => s !== scrollbarSheet)
        originalSet.call(this, [...filtered, scrollbarSheet])
      },
      get: descriptor.get
    })
  } else {
    const originalAttachShadow = Element.prototype.attachShadow
    Element.prototype.attachShadow = function (init) {
      const root = originalAttachShadow.call(this, init)
      setTimeout(() => {
        root.adoptedStyleSheets = [...root.adoptedStyleSheets, scrollbarSheet]
      }, 0)
      return root
    }
  }
}

// 引入 Material Web Components
import '@material/web/button/filled-button.js'
import '@material/web/button/outlined-button.js'
import '@material/web/iconbutton/icon-button.js'
import '@material/web/iconbutton/filled-icon-button.js'
import '@material/web/iconbutton/filled-tonal-icon-button.js'
import '@material/web/textfield/filled-text-field.js'
import '@material/web/textfield/outlined-text-field.js'
import '@material/web/checkbox/checkbox.js'
import '@material/web/select/filled-select.js'
import '@material/web/select/select-option.js'
import '@material/web/dialog/dialog.js'
import '@material/web/switch/switch.js'
import '@material/web/button/text-button.js'

createApp(App).mount('#app')