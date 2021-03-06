import { useIntl } from 'react-intl'

import { Layout } from '@/components/layout'
import { messages as menuT } from '@/components/menu/menu-i18n'

export const Notifications = () => {
  const { formatMessage: _ } = useIntl()

  return (
    <Layout
      title={_(menuT.notifications)}
      breadcrumbs={[
        {
          to: '/',
          label: _(menuT.dashboard),
        },
        {
          label: _(menuT.notifications),
        },
      ]}
    >
      <div />
    </Layout>
  )
}
