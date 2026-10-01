import {
  Alert, Badge, Button, Card, Checkbox, Code, CopyButton, Group, MultiSelect,
  NumberInput, PasswordInput, SimpleGrid, Stack, Table, Text, Anchor, Modal,
} from '@mantine/core'
import {
  IconAlertCircle, IconCalendarEvent, IconCheck, IconCopy, IconPlayerPlay,
  IconRefresh, IconSettings, IconTrophy,
} from '@tabler/icons-react'
import { notifications } from '@mantine/notifications'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useRef, useState } from 'react'
import Hls from 'hls.js'

import {
  sportsApi,
  type SportsAutomationSettings,
  type SportsAutomationView,
  type SportsEventChannel,
} from '../api/sports'

function absoluteFeedURL(path: string) {
  return `${window.location.origin}${path}`
}

function SportsPlayer({ channel }: { channel: SportsEventChannel }) {
  const videoRef = useRef<HTMLVideoElement>(null)

  useEffect(() => {
    const video = videoRef.current
    if (!video) return
    let hls: Hls | null = null
    if (Hls.isSupported()) {
      hls = new Hls({ enableWorker: false })
      hls.loadSource(channel.play_url)
      hls.attachMedia(video)
    } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
      video.src = channel.play_url
    }
    return () => {
      hls?.destroy()
      video.removeAttribute('src')
      video.load()
    }
  }, [channel])

  return (
    <Stack gap="xs">
      <video
        ref={videoRef}
        controls
        autoPlay
        playsInline
        style={{ width: '100%', aspectRatio: '16 / 9', background: '#000' }}
      />
      <Text size="xs" c="dimmed">
        Tunerr stream · {channel.source_channel_name}
        {channel.source_guide_number ? ` · channel ${channel.source_guide_number}` : ''}
      </Text>
    </Stack>
  )
}

function eventTime(value: string) {
  return new Date(value).toLocaleString([], { weekday: 'short', month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' })
}

export function Sports() {
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState<SportsAutomationSettings | null>(null)
  const [dirty, setDirty] = useState(false)
  const [playing, setPlaying] = useState<SportsEventChannel | null>(null)
  const [apiKeyDraft, setApiKeyDraft] = useState('')
  const query = useQuery({
    queryKey: ['sports-automation'],
    queryFn: () => sportsApi.automation(),
    refetchInterval: 60_000,
    refetchOnWindowFocus: true,
    retry: false,
  })

  useEffect(() => {
    if (query.data && !dirty) setDraft(query.data.settings)
  }, [query.data, dirty])

  const save = useMutation({
    mutationFn: (settings: SportsAutomationSettings) => sportsApi.save(settings),
    onSuccess: result => {
      setDraft(result.settings)
      setDirty(false)
      void queryClient.invalidateQueries({ queryKey: ['sports-automation'] })
      notifications.show({ title: 'Sports automation saved', message: 'Tunerr will use these rules during schedule refresh.', color: 'teal' })
    },
    onError: error => notifications.show({ title: 'Could not save settings', message: String(error), color: 'red' }),
  })

  const saveAPIKey = useMutation({
    mutationFn: (key: string | null) => key === null ? sportsApi.removeKey() : sportsApi.saveKey(key),
    onSuccess: result => {
      setApiKeyDraft('')
      queryClient.setQueryData(['sports-automation'], result.view)
      void queryClient.invalidateQueries({ queryKey: ['sports-automation'] })
      notifications.show({
        title: result.view.api_key_configured ? 'API-Sports key saved' : 'API-Sports key removed',
        message: result.view.api_key_configured
          ? 'Sports Automation can use the key now. It is stored on the server and will not be shown again.'
          : 'Sports Automation is disconnected from API-Sports.',
        color: 'teal',
      })
    },
    onError: error => notifications.show({ title: 'Could not update API-Sports key', message: String(error), color: 'red' }),
  })

  const refresh = useMutation({
    mutationFn: () => sportsApi.refresh(),
    onSuccess: result => {
      void queryClient.invalidateQueries({ queryKey: ['sports-automation'] })
      notifications.show({
        title: result.warning ? 'Schedule refresh finished with stale data' : 'Schedules refreshed',
        message: result.warning || `${result.report.schedule_event_count} games checked; ${result.report.generated_count} event feeds generated.`,
        color: result.warning ? 'yellow' : 'teal',
      })
    },
    onError: error => notifications.show({ title: 'Schedule refresh failed', message: String(error), color: 'red' }),
  })

  const view: SportsAutomationView | undefined = query.data
  const settings = draft ?? view?.settings
  const teamOptions = useMemo(() => {
    const teams = new Map<string, string>()
    for (const option of view?.available_teams ?? []) {
      const key = `${option.dataset}|${option.team}`
      const label = `${view?.datasets.find(item => item.id === option.dataset)?.name ?? option.dataset} · ${option.team}`
      teams.set(key, label)
    }
    for (const rule of settings?.team_rules ?? []) {
      const key = `${rule.dataset}|${rule.team}`
      const label = `${view?.datasets.find(item => item.id === rule.dataset)?.name ?? rule.dataset} · ${rule.team}`
      teams.set(key, label)
    }
    return [...teams.entries()].sort((a, b) => a[1].localeCompare(b[1])).map(([value, label]) => ({ value, label }))
  }, [view?.available_teams, view?.datasets, settings?.team_rules])

  function update(next: Partial<SportsAutomationSettings>) {
    if (!settings) return
    setDraft({ ...settings, ...next })
    setDirty(true)
  }

  function setDataset(id: string, checked: boolean) {
    if (!settings) return
    const datasets = new Set(settings.datasets)
    if (checked) datasets.add(id)
    else datasets.delete(id)
    update({ datasets: [...datasets].sort() })
  }

  function setTeamRules(values: string[]) {
    const rules = values.flatMap(value => {
      const separator = value.indexOf('|')
      if (separator < 0) return []
      return [{ dataset: value.slice(0, separator), team: value.slice(separator + 1) }]
    })
    update({ team_rules: rules })
  }

  const selectedTeamValues = settings?.team_rules?.map(rule => `${rule.dataset}|${rule.team}`) ?? []
  const m3uURL = absoluteFeedURL(sportsApi.playlistURL)
  const guideURL = absoluteFeedURL(sportsApi.guideURL)

  if (query.isLoading) return <Text c="dimmed">Loading Sports Automation…</Text>
  if (query.isError || !view || !settings) {
    return (
      <Alert icon={<IconAlertCircle size={16} />} color="red" title="Sports Automation unavailable">
        The tuner API did not return sports settings. Confirm that Tunerr is running and the WebUI proxy is connected.
      </Alert>
    )
  }

  const report = view.report
  const status = view.status

  return (
    <Stack gap="lg">
      <Group justify="space-between" align="flex-end" wrap="wrap">
        <Stack gap={2}>
          <Text size="xl" fw={700}>Sports Automation</Text>
          <Text size="sm" c="dimmed">
            Match API-Sports schedules to games in your provider guide and publish temporary Tunerr event channels.
          </Text>
        </Stack>
        <Group gap="xs">
          <Button
            variant="default"
            leftSection={<IconRefresh size={15} />}
            loading={refresh.isPending}
            disabled={!settings.enabled || !settings.datasets.length || !view.api_key_configured || dirty}
            onClick={() => refresh.mutate()}
          >
            Refresh schedules
          </Button>
          <Button
            leftSection={<IconSettings size={15} />}
            loading={save.isPending}
            disabled={!dirty || !view.settings_writable}
            onClick={() => save.mutate(settings)}
          >
            Save rules
          </Button>
        </Group>
      </Group>

      <Card withBorder padding="md" radius="md">
        <Stack gap="sm">
          <Group justify="space-between" wrap="wrap">
            <Text fw={650}>API-Sports connection</Text>
            <Badge color={view.api_key_configured ? 'teal' : 'gray'} variant="light">
              {view.api_key_configured ? 'Connected' : 'Not connected'}
            </Badge>
          </Group>
          {view.api_key_writable ? (
            <>
              <Text size="sm" c="dimmed">
                Paste your API-Sports key here. Tunerr saves it as an owner-only file in its state directory and starts using it immediately; the saved key is never shown again.
              </Text>
              <PasswordInput
                label={view.api_key_configured ? 'Replace API-Sports key' : 'API-Sports key'}
                placeholder={view.api_key_configured ? 'Enter a new key to replace the saved one' : 'Paste your API-Sports key'}
                autoComplete="new-password"
                value={apiKeyDraft}
                onChange={event => setApiKeyDraft(event.currentTarget.value)}
              />
              <Group>
                <Button
                  leftSection={<IconTrophy size={15} />}
                  loading={saveAPIKey.isPending}
                  disabled={!apiKeyDraft.trim()}
                  onClick={() => saveAPIKey.mutate(apiKeyDraft)}
                >
                  Save API-Sports key
                </Button>
                {view.api_key_configured && (
                  <Button
                    variant="default"
                    loading={saveAPIKey.isPending}
                    onClick={() => {
                      if (window.confirm('Remove the saved API-Sports key?')) saveAPIKey.mutate(null)
                    }}
                  >
                    Remove saved key
                  </Button>
                )}
              </Group>
            </>
          ) : (
            <Alert color={view.api_key_source === 'environment' ? 'teal' : 'blue'} title={view.api_key_source === 'environment' ? 'Key supplied by the server environment' : 'Connect API-Sports'} icon={<IconTrophy size={16} />}>
              {view.api_key_source === 'environment'
                ? <>The key comes from <Code>IPTV_TUNERR_API_SPORTS_KEY</Code>. Change it in your container or service environment, then restart Tunerr.</>
                : <>This instance has no writable key file. Set <Code>IPTV_TUNERR_API_SPORTS_KEY</Code> in the Tunerr server environment and restart it. The key is never returned by the API.</>}
            </Alert>
          )}
        </Stack>
      </Card>
      {!view.settings_writable && (
        <Alert color="yellow" title="Settings are read-only">
          Set <Code>IPTV_TUNERR_SPORTS_AUTOMATION_FILE</Code> to a writable state file to save automation rules.
        </Alert>
      )}

      {report.warnings?.map(warning => (
        <Alert key={warning} color="yellow" icon={<IconAlertCircle size={16} />} title="Schedule or guide warning">
          {warning}
        </Alert>
      ))}

      <SimpleGrid cols={{ base: 1, md: 2 }} spacing="md">
        <Card withBorder padding="md" radius="md">
          <Stack gap="sm">
            <Group justify="space-between">
              <Text fw={650}>Schedule sources</Text>
              <Badge color={settings.enabled ? 'teal' : 'gray'} variant="light">{settings.enabled ? 'Automation on' : 'Automation off'}</Badge>
            </Group>
            <Checkbox
              label="Enable Sports Automation"
              checked={settings.enabled}
              onChange={event => update({ enabled: event.currentTarget.checked })}
            />
            <Stack gap="xs">
              {view.datasets.map(dataset => (
                <Checkbox
                  key={dataset.id}
                  label={dataset.name}
                  description={`${dataset.product} · league ID ${dataset.remote_league_id}`}
                  checked={settings.datasets.includes(dataset.id)}
                  onChange={event => setDataset(dataset.id, event.currentTarget.checked)}
                />
              ))}
            </Stack>
            <Text size="xs" c="dimmed">
              These are the API-Sports schedule datasets supported by the reference integration. Other sports continue to use your provider and XMLTV feeds without API-Sports requests.
            </Text>
          </Stack>
        </Card>

        <Card withBorder padding="md" radius="md">
          <Stack gap="sm">
            <Text fw={650}>Event rules</Text>
            <MultiSelect
              label="Follow teams (optional)"
              placeholder={teamOptions.length ? 'All teams are included when empty' : 'Refresh schedules to load team choices'}
              data={teamOptions}
              value={selectedTeamValues}
              onChange={setTeamRules}
              searchable
              clearable
              nothingFoundMessage="No teams in the current schedule cache"
              disabled={!settings.enabled || !settings.datasets.length}
            />
            <Group grow align="flex-start">
              <NumberInput
                label="Keep events after start (hours)"
                min={0}
                max={12}
                value={settings.past_hours}
                onChange={value => update({ past_hours: Number(value) || 0 })}
              />
              <NumberInput
                label="Schedule lookahead (days)"
                min={1}
                max={7}
                value={settings.lookahead_days}
                onChange={value => update({ lookahead_days: Number(value) || 1 })}
              />
              <NumberInput
                label="Feeds per game"
                min={1}
                max={12}
                value={settings.max_channels_per_event}
                onChange={value => update({ max_channels_per_event: Number(value) || 1 })}
              />
            </Group>
            <Text size="xs" c="dimmed">
              A game is generated only when its channel or EPG programme names both teams and the programme starts within 3 hours of the canonical schedule.
            </Text>
          </Stack>
        </Card>
      </SimpleGrid>

      <Card withBorder padding="md" radius="md">
        <Stack gap="sm">
          <Group justify="space-between" wrap="wrap">
            <Text fw={650}>API-Sports cache health</Text>
            <Text size="xs" c="dimmed">
              {status.last_success_at ? `Last schedule response ${new Date(status.last_success_at).toLocaleString()}` : 'No API-Sports schedule has been cached yet'}
            </Text>
          </Group>
          <Table.ScrollContainer minWidth={600}>
            <Table striped highlightOnHover withTableBorder>
              <Table.Thead>
                <Table.Tr><Table.Th>Dataset</Table.Th><Table.Th>Cached dates</Table.Th><Table.Th>Events</Table.Th><Table.Th>Stale dates</Table.Th><Table.Th>Last fetch</Table.Th></Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {status.datasets.map(item => (
                  <Table.Tr key={item.id}>
                    <Table.Td>{view.datasets.find(dataset => dataset.id === item.id)?.name ?? item.id}</Table.Td>
                    <Table.Td>{item.cached_dates.length ? item.cached_dates.join(', ') : '—'}</Table.Td>
                    <Table.Td>{item.cached_event_count}</Table.Td>
                    <Table.Td>{item.stale_date_count ? <Badge color="yellow">{item.stale_date_count}</Badge> : '0'}</Table.Td>
                    <Table.Td>{item.last_fetch_at ? new Date(item.last_fetch_at).toLocaleString() : '—'}</Table.Td>
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          </Table.ScrollContainer>
          {status.quota.daily_remaining !== undefined && (
            <Text size="xs" c="dimmed">API-Sports daily quota remaining: {status.quota.daily_remaining}{status.quota.daily_limit !== undefined ? ` / ${status.quota.daily_limit}` : ''}</Text>
          )}
        </Stack>
      </Card>

      <Card withBorder padding="md" radius="md">
        <Stack gap="md">
          <Group justify="space-between" wrap="wrap">
            <Stack gap={1}>
              <Text fw={650}>Generated event channels</Text>
              <Text size="xs" c="dimmed">
                {report.matched_event_count} of {report.schedule_event_count} scheduled games matched · {report.generated_count} playable feeds
              </Text>
            </Stack>
            <Group gap="xs">
              <CopyButton value={m3uURL} timeout={1500}>
                {({ copied, copy }) => <Button size="xs" variant="default" leftSection={copied ? <IconCheck size={14} /> : <IconCopy size={14} />} onClick={() => { void copy() }}>{copied ? 'Copied M3U URL' : 'Copy M3U URL'}</Button>}
              </CopyButton>
              <CopyButton value={guideURL} timeout={1500}>
                {({ copied, copy }) => <Button size="xs" variant="default" leftSection={<IconCopy size={14} />} onClick={() => { void copy() }}>{copied ? 'Copied guide URL' : 'Copy guide URL'}</Button>}
              </CopyButton>
            </Group>
          </Group>
          <Group gap="xs">
            <Anchor href={sportsApi.playlistURL} target="_blank" size="xs">Open sports M3U</Anchor>
            <Text size="xs" c="dimmed">·</Text>
            <Anchor href={sportsApi.guideURL} target="_blank" size="xs">Open sports XMLTV</Anchor>
          </Group>
          <Text size="xs" c="dimmed">
            Generated feeds have separate identities and reusable channel numbers. Playback always goes through a matched channel in Tunerr's current lineup; the regular lineup and its manual channels are unchanged.
          </Text>
          {report.events.length === 0 ? (
            <Alert color="gray" icon={<IconCalendarEvent size={16} />} title="No scheduled games in this window">
              Enable a dataset and refresh schedules. Generated channels also require a ready Tunerr guide with a programme naming both teams.
            </Alert>
          ) : (
            <Stack gap="sm">
              {report.events.map(row => (
                <Card key={row.event.id} withBorder padding="sm" radius="sm">
                  <Group justify="space-between" align="flex-start" wrap="wrap">
                    <Stack gap={4} style={{ minWidth: 240, flex: 1 }}>
                      <Group gap="xs">
                        <Badge size="sm" variant="light">{view.datasets.find(item => item.id === row.event.dataset)?.name ?? row.event.dataset}</Badge>
                        <Text size="xs" c="dimmed">{eventTime(row.event.starts_at)}</Text>
                      </Group>
                      <Text fw={600}>{row.event.away_team} at {row.event.home_team}</Text>
                      {row.matched ? (
                        <Text size="xs" c="teal">{row.channels?.length ?? 0} Tunerr feed{row.channels?.length === 1 ? '' : 's'} matched</Text>
                      ) : (
                        <Text size="xs" c="dimmed">No feed match: {row.reason}</Text>
                      )}
                    </Stack>
                    {!!row.channels?.length && (
                      <Group gap="xs" wrap="wrap">
                        {row.channels.map(channel => (
                          <Button
                            key={channel.id}
                            size="xs"
                            variant="light"
                            color="teal"
                            leftSection={<IconPlayerPlay size={13} />}
                            onClick={() => setPlaying(channel)}
                          >
                            {channel.source_channel_name || 'Play feed'}
                          </Button>
                        ))}
                      </Group>
                    )}
                  </Group>
                </Card>
              ))}
            </Stack>
          )}
        </Stack>
      </Card>

      <Modal opened={playing !== null} onClose={() => setPlaying(null)} title={playing?.name ?? 'Sports event'} size="xl" centered>
        {playing && <SportsPlayer channel={playing} />}
      </Modal>
    </Stack>
  )
}
