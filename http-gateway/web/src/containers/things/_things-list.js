import PropTypes from 'prop-types'
import { useMemo } from 'react'
import { useIntl } from 'react-intl'
import { Link } from 'react-router-dom'
import classNames from 'classnames'

import { Badge } from '@/components/badge'
import { Table } from '@/components/table'

import {
  thingsStatuses,
  THINGS_DEFAULT_PAGE_SIZE,
  NO_DEVICE_NAME,
} from './constants'
import { thingShape } from './shapes'
import { messages as t } from './things-i18n'

const { ONLINE, UNREGISTERED } = thingsStatuses

export const ThingsList = ({ data }) => {
  const { formatMessage: _ } = useIntl()

  const columns = useMemo(
    () => [
      {
        Header: _(t.name),
        accessor: 'device.n',
        Cell: ({ value, row }) => {
          const deviceName = value || NO_DEVICE_NAME

          if (row.original?.status === UNREGISTERED) {
            return <span>{deviceName}</span>
          }
          return <Link to={`/things/${row.original?.device?.di}`}>{deviceName}</Link>
        },
        style: { width: '33%' },
      },
      {
        Header: 'ID',
        accessor: 'device.di',
        Cell: ({ value }) => {
          return <span className="no-wrap-text">{value}</span>
        },
      },
      {
        Header: _(t.status),
        accessor: 'status',
        style: { width: '120px' },
        Cell: ({ value }) => {
          const isOnline = ONLINE === value
          return (
            <Badge className={isOnline ? 'green' : 'red'}>
              {isOnline ? _(t.online) : _(t.offline)}
            </Badge>
          )
        },
      },
    ],
    [] //eslint-disable-line
  )

  return (
    <Table
      columns={columns}
      data={data || []}
      defaultSortBy={[
        {
          id: 'device.n',
          desc: false,
        },
      ]}
      autoFillEmptyRows
      defaultPageSize={THINGS_DEFAULT_PAGE_SIZE}
      getRowProps={row => ({
        className: classNames({
          'grayed-out': row.original?.status === UNREGISTERED,
        }),
      })}
    />
  )
}

ThingsList.propTypes = {
  data: PropTypes.arrayOf(thingShape),
}

ThingsList.defaultProps = {
  data: [],
}
