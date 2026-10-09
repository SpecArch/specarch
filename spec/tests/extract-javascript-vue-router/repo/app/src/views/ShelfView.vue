<template>
  <h1>Shelves</h1>
  <DataTable :value="shelves">
    <Column field="name" header="Name" />
    <Column field="room" header="Room" />
  </DataTable>
</template>

<script setup lang="ts">
import Column from 'primevue/column';
import DataTable from 'primevue/datatable';

const shelves = await $fetch('/api/shelves');
</script>
