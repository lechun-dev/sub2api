<template>
  <AppLayout><div class="mx-auto max-w-6xl p-4 sm:p-6"><ClientAccessManager ref="manager" :group-id="groupId" /></div></AppLayout>
</template>
<script setup lang="ts">
import { computed, ref } from 'vue'
import { onBeforeRouteLeave, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ClientAccessManager from '@/components/admin/ClientAccessManager.vue'
const route = useRoute()
const { t } = useI18n()
const manager = ref<InstanceType<typeof ClientAccessManager> | null>(null)
const groupId = computed(() => typeof route.query.group === 'string' ? route.query.group : undefined)
onBeforeRouteLeave(() => !manager.value?.dirty || window.confirm(t('admin.clientAccess.discard')))
</script>
