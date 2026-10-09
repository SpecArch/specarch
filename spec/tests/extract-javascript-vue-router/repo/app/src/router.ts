import { createRouter, createWebHistory } from 'vue-router';
import { requirePermission } from './auth';
import ShelfView from './views/ShelfView.vue';
import { Dashboard } from 'some-dashboard';

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/shelves', component: ShelfView },
    { path: '/shelves/new', component: () => import('./views/ShelfForm.vue'), beforeEnter: () => requirePermission('shelves.write') },
    { path: '/about', component: () => import('./views/AboutView.vue') },
    { path: '/dashboard', component: Dashboard },
  ],
});

router.beforeEach(() => requirePermission('shelves.read'));

export default router;
