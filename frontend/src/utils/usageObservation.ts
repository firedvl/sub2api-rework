/** Response observation completeness does not establish final provider spend. */
export function usageObservationKey(complete: boolean | null | undefined): string {
  return complete === true ? 'usage.observationComplete' : complete === false ? 'usage.observationPartial' : 'usage.observationUnknown'
}
