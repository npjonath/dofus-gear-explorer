import struct
import unittest
from uuid import UUID

from combat_bank import Bank, SETUPS, Sample, modes, ogg_crc, remux


class SoundBankTest(unittest.TestCase):
    def test_event_graph_preserves_simultaneous_layers_and_alternative_takes(self):
        bank = Bank(None)
        event, timeline, instrument, wave1, wave2, wave3, sample1, sample2 = [UUID(int=i).bytes_le for i in range(1,9)]
        bank.events[event] = timeline
        bank.timelines[timeline] = [(instrument, 0), (wave3, .5)]
        bank.instruments[instrument] = ("choices", [wave1, wave2])
        bank.instruments[wave1] = ("wave", sample1)
        bank.instruments[wave2] = ("wave", sample2)
        bank.instruments[wave3] = ("wave", sample2)
        bank.waves = {sample1: (0,1), sample2: (0,2)}
        self.assertEqual(bank.clips(UUID(bytes_le=event).hex), [(0,[(0,1),(0,2)]),(.5,[(0,2)])])

    def test_remux_preserves_packets_and_writes_valid_vorbis_pages(self):
        # Synthetic packet contents; no game samples are needed in the tests.
        crc = 3626110227
        packets = [bytes([2, 0, i]) for i in range(3)]
        fsb = b"".join(struct.pack("<H",len(p)) + p for p in packets) + b"\0\0"
        sample = Sample("fixture",0,len(fsb),1,44100,2000,crc)
        ogg = remux(fsb,sample)
        self.assertEqual(modes(SETUPS[crc]),[False,True])
        pages = []
        at = 0
        while at < len(ogg):
            self.assertEqual(ogg[at:at+4],b"OggS")
            n = ogg[at+26]
            begin = at + 27 + n
            end = begin + sum(ogg[at+27:begin])
            data = bytearray(ogg[at:end])
            checksum, = struct.unpack_from("<I",data,22)
            data[22:26] = b"\0"*4
            self.assertEqual(ogg_crc(data),checksum)
            pages.append((ogg[at+5], struct.unpack_from("<q",ogg,at+6)[0],ogg[begin:end]))
            at = end
        self.assertEqual([p[0] for p in pages],[2,0,4])
        self.assertTrue(pages[0][2].startswith(b"\x01vorbis"))
        self.assertIn(SETUPS[crc],pages[1][2])
        self.assertEqual(pages[-1][1:],(2000,b"".join(packets)))
        self.assertIsNone(remux(fsb,Sample("unknown",0,len(fsb),1,44100,2000,0)))


if __name__ == "__main__":
    unittest.main()
