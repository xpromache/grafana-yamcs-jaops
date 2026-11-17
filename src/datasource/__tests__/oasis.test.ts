import { DataQueryResponse, FieldType, toDataFrame } from '@grafana/data';
import { matchesEndpoint, nameByEndpoint, parseEndpointsQuery, splitEndpoints } from '../datasource';
import { endpointSlug } from '../config/ConfigEditor';
import { aliasesToText, textToAliases } from '../config/ConfigHost';

describe('endpoint variables', () => {
    it('splits csv and braces', () => {
        expect(splitEndpoints('sat-1')).toEqual(['sat-1']);
        expect(splitEndpoints('sat-1,sat-2')).toEqual(['sat-1', 'sat-2']);
        expect(splitEndpoints('{sat-1, sat-2}')).toEqual(['sat-1', 'sat-2']);
        expect(splitEndpoints('')).toEqual([]);
    });

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

    it('names value fields after the endpoint', () => {
        const frame = toDataFrame({
            fields: [
                { name: 'time', type: FieldType.time, values: [1] },
                { name: '/HK/POWER/soc', type: FieldType.number, values: [80] },
            ],
        });
        const res = nameByEndpoint({ data: [frame] } as DataQueryResponse, 'sat-2');
        const field = res.data[0].fields[1];
        expect(field.config.displayNameFromDS).toBe('sat-2 · soc');
        expect(field.labels).toEqual({ satellite: 'sat-2' });
        expect(res.data[0].fields[0].config.displayNameFromDS).toBeUndefined();
    });
});

describe('config editor helpers', () => {
    it('slugs instance names', () => {
        expect(endpointSlug('sat-1')).toBe('sat-1');
        expect(endpointSlug(' my sat/1 ')).toBe('my-sat-1');
    });

    it('round-trips aliases', () => {
        expect(textToAliases('fm1=sat-1\n bad \nfm2 = sat-2')).toEqual({ fm1: 'sat-1', fm2: 'sat-2' });
        expect(aliasesToText({ fm1: 'sat-1' })).toBe('fm1=sat-1');
    });
});
