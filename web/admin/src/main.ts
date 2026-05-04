import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'

import {
  Clock,
  Connection,
  DataAnalysis,
  Finished,
  Goods,
  Key,
  List,
  Monitor,
  Operation,
  Refresh,
  Search,
  Setting,
  SwitchButton,
  TrendCharts,
  View,
} from '@element-plus/icons-vue'
import { createApp } from 'vue'

import App from './App.vue'
import './styles.css'

const app = createApp(App)

app.use(ElementPlus)

for (const icon of [
  Clock,
  Connection,
  DataAnalysis,
  Finished,
  Goods,
  Key,
  List,
  Monitor,
  Operation,
  Refresh,
  Search,
  Setting,
  SwitchButton,
  TrendCharts,
  View,
]) {
  app.component(icon.name!, icon)
}

app.mount('#app')
