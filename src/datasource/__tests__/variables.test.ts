import { matchesEndpoint, parseEndpointsQuery } from '../datasource';

describe('endpoint variables', () => {
    it('parses endpoints() queries', () => {
        expect(parseEndpointsQuery('endpoints()')).toEqual({});
        expect(parseEndpointsQuery(' endpoints(mdb=pussim, host=yamcs-a) ')).toEqual({
            mdb: 'pussim',
            host: 'yamcs-a',
        });
        expect(parseEndpointsQuery('parameters()')).toBeNull();
    });

    it('filters by label, host and instance', () => {
        const info = {
            online: true,
            name: 'sat-1',
            description: '',
            host: 'yamcs-a',
            instance: 'sat-1',
            labels: { mdb: 'pussim' },
        };
        expect(matchesEndpoint(info, {})).toBe(true);
        expect(matchesEndpoint(info, { mdb: 'pussim', host: 'yamcs-a' })).toBe(true);
        expect(matchesEndpoint(info, { mdb: 'other' })).toBe(false);
        expect(matchesEndpoint(info, { instance: 'sat-2' })).toBe(false);
    });
});
