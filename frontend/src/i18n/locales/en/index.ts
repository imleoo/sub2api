import landing from './landing'
import common from './common'
import dashboard from './dashboard'
import admin from './admin'
import misc from './misc'
import fork from './fork'
import { deepMergeLocale } from '../forkMerge'

// fork（zhiguofan）自定义键最后深合并覆盖，见 ./fork.ts
export default deepMergeLocale({
  ...landing,
  ...common,
  ...dashboard,
  admin,
  ...misc,
}, fork)
