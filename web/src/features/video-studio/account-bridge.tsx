import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useCallback, useEffect, useState, type RefObject } from 'react'

import { SignOutDialog } from '@/components/sign-out-dialog'
import { getUserProfile } from '@/features/profile/api'
import { formatQuota } from '@/lib/format'
import { useAuthStore } from '@/stores/auth-store'
import { useSystemConfigStore } from '@/stores/system-config-store'

export function VideoStudioAccountBridge({
  frameRef,
}: {
  frameRef: RefObject<HTMLIFrameElement | null>
}) {
  const navigate = useNavigate()
  const userId = useAuthStore((state) => state.auth.user?.id)
  const sid = useAuthStore((state) => state.auth.session?.sid)
  useSystemConfigStore((state) => state.config.currency)
  const [signOutOpen, setSignOutOpen] = useState(false)
  const account = useQuery({
    queryKey: ['video-studio-account', userId, sid],
    enabled: Boolean(userId),
    retry: false,
    refetchOnMount: 'always',
    refetchOnWindowFocus: 'always',
    queryFn: async () => {
      const response = await getUserProfile()
      const data = response.data
      const auth = useAuthStore.getState().auth
      if (
        !response.success ||
        !data ||
        data.id !== userId ||
        auth.user?.id !== userId ||
        auth.session?.sid !== sid ||
        !Number.isFinite(data.quota) ||
        !Number.isFinite(data.used_quota)
      ) {
        throw new Error('Invalid account response')
      }
      auth.setUser({
        ...auth.user,
        quota: data.quota,
        used_quota: data.used_quota,
        request_count: data.request_count,
        group: data.group,
      })
      return data
    },
  })
  const { data, isError, isFetching, refetch } = account
  const balance =
    !isError && !isFetching && data ? formatQuota(data.quota) : null
  const used =
    !isError && !isFetching && data ? formatQuota(data.used_quota) : null
  const sendAccount = useCallback(() => {
    if (!userId) return
    const profile = isError ? undefined : data
    frameRef.current?.contentWindow?.postMessage(
      {
        type: 'new-api-video-account',
        account: {
          user_id: userId,
          username: profile?.username ?? '',
          display_name: profile?.display_name ?? '',
          group: profile?.group ?? '',
          balance,
          used,
          loading: isFetching,
        },
      },
      window.location.origin
    )
  }, [data, isError, isFetching, userId, frameRef, balance, used])

  useEffect(() => {
    sendAccount()
  }, [sendAccount])

  useEffect(() => {
    const refresh = () => {
      void refetch()
    }
    const onMessage = (event: MessageEvent) => {
      if (
        event.origin !== window.location.origin ||
        !frameRef.current ||
        event.source !== frameRef.current.contentWindow
      ) {
        return
      }
      if (event.data?.type === 'new-api-video-ready') {
        sendAccount()
      } else if (event.data?.type === 'new-api-video-account-changed') {
        refresh()
      } else if (event.data?.type === 'new-api-video-account-action') {
        if (event.data.action === 'sign-out') setSignOutOpen(true)
        if (event.data.action === 'dashboard') {
          void navigate({ to: '/dashboard' })
        }
      }
    }
    window.addEventListener('focus', refresh)
    window.addEventListener('message', onMessage)
    return () => {
      window.removeEventListener('focus', refresh)
      window.removeEventListener('message', onMessage)
    }
  }, [frameRef, refetch, sendAccount, navigate])

  return <SignOutDialog open={signOutOpen} onOpenChange={setSignOutOpen} />
}
