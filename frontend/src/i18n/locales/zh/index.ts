import landing from './landing'
import common from './common'
import dashboard from './dashboard'
import admin from './admin'
import misc from './misc'
import devzz from './devzz'
import enterpriseMembers from './enterpriseMembers'
import { mergeLocale } from '../mergeLocale'

const upstreamLocale = {
  ...landing,
  ...common,
  ...dashboard,
  admin,
  ...misc,
  enterpriseMembers: {
    ...common.enterpriseMembers,
    ...enterpriseMembers,
  },
}

export default mergeLocale(upstreamLocale, devzz)
