import { z } from 'zod'

/** URL search params for the Data Explorer (kept minimal and shareable). */
export const dataSearchSchema = z.object({
	db: z.string().optional(),
	schema: z.string().optional(),
	table: z.string().optional(),
	page: z.number().int().positive().catch(1).optional(),
})

export type DataSearch = z.infer<typeof dataSearchSchema>
