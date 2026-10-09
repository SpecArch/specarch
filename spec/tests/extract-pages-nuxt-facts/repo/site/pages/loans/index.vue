<template>
  <main>
    <h1>Loans</h1>
    <DataTable :value="loans">
      <Column field="book" header="Book" />
      <Column field="due" header="Due" />
      <Column :field="extra" header="Extra" />
    </DataTable>
  </main>
</template>

<script setup lang="ts">
import Column from 'primevue/column';
import DataTable from 'primevue/datatable';

definePageMeta({ title: 'All loans', middleware: 'auth', layout: 'desk', keepalive: true });
const extra = 'member';
const loans = await $fetch('/api/loans');
</script>
