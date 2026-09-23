import type { Goal, FinishRoute } from './types';

export interface AdminUser {
  id: string;
  username: string;
}
export interface AdminStatus {
  configured: boolean;
  user: AdminUser | null;
}
export interface CatalogDocument {
  bowserRoutes?: FinishRoute[];
  goals: Goal[];
  routeAreaTimes?: Record<string, number>;
  revision: string;
}
export interface DraftGoal extends Omit<Goal, 'tags' | 'routeAreas' | 'conflictGroups'> {
  key: number;
  areas: string;
  kinds: string;
  routes: string[];
  groups: string;
}

export interface DraftRouteArea {
  key: number;
  name: string;
  timeMin: number | '';
}
