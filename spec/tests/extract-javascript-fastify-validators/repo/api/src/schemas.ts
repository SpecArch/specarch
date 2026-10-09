import * as yup from 'yup';
import Joi from 'joi';
import { z } from 'zod';

export const noteSchema = yup.object({
  title: yup.string().required().max(120),
  pinned: yup.boolean(),
  colour: yup.string().oneOf(['plain', 'yellow', 'blue']),
  due: yup.date().nullable(),
  tags: yup.array().of(yup.string()),
});

export const commentSchema = Joi.object({
  text: Joi.string().min(1).max(500).required(),
  author: Joi.string().email(),
  rating: Joi.number().integer().min(1).max(5),
  visibility: Joi.string().valid('public', 'team'),
});

export const searchSchema = z.object({
  q: z.string().min(2),
  page: z.number().int().positive().optional(),
  sort: z.enum(['newest', 'oldest']).default('newest'),
  filter: z.union([z.string(), z.number()]),
  slug: z.string().regex(/^[a-z-]+$/),
  from: z.coerce.date(),
  colour: colourSchema,
});

export const colourSchema = z.enum(['plain', 'yellow', 'blue']);
