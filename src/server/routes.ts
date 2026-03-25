import type { FastifyInstance } from 'fastify';
import type { Handlers } from './handlers.js';

export function registerRoutes(app: FastifyInstance, handlers: Handlers): void {
  app.post('/api/todo', handlers.createTodo.bind(handlers));
  app.get('/api/todo', handlers.listTodos.bind(handlers));
  app.put('/api/todo/:id', handlers.updateTodo.bind(handlers));
  app.delete('/api/todo/:id', handlers.deleteTodo.bind(handlers));
  app.patch('/api/todo/:id/status', handlers.changeStatus.bind(handlers));
  app.post('/api/dump', handlers.bulkDump.bind(handlers));
  app.get('/api/info', handlers.getInfo.bind(handlers));
  app.get('/api/health', handlers.healthCheck.bind(handlers));
}
