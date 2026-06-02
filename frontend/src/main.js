import './style.css'
import { mount } from 'svelte'
import App from './App.svelte'

// @ts-ignore - Wails generated bindings
import * as Backend from '../wailsjs/go/main/App.js'

window.backend = Backend

const app = mount(App, {
  target: /** @type {Element} */ (document.getElementById('app'))
})

export default app