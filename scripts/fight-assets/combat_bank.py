"""Read the FEV event and FSB5 sample tables used by the installed desktop client.

The in-game Vorbis packets are remultiplexed into Ogg without decoding audio or
loading a native plugin. The setup packet is selected by its FSB5 CRC.
"""
import mmap
import struct
import zlib
from dataclasses import dataclass
from uuid import UUID

from fsb5.vorbis_headers import lookup as SETUPS


U16 = struct.Struct("<H")
U32 = struct.Struct("<I")
REVERSE = bytes(int(f"{n:08b}"[::-1], 2) for n in range(256))
RATES = (4000, 8000, 11000, 11025, 16000, 22050, 24000, 32000, 44100, 48000, 96000)


def u16(data, offset):
    return U16.unpack_from(data, offset)[0]


def u32(data, offset):
    return U32.unpack_from(data, offset)[0]


def size_value(data, at):
    count = u16(data, at)
    at += 2
    if count & 0x8000:
        count = (count & 0x7fff) | u16(data, at) << 15
        at += 2
    return count, at


def integer_array(data, at):
    count, at = size_value(data, at)
    count //= 2
    if not count:
        return [], at
    size = u16(data, at)
    at += 2
    if size == 0 or at + count * size > len(data):
        raise ValueError("Invalid FEV array")
    return [data[at + i * size:at + (i + 1) * size] for i in range(count)], at + size * count


@dataclass
class Sample:
    name: str
    start: int
    end: int
    channels: int
    rate: int
    frames: int
    crc: int


class Bank:
    def __init__(self, path):
        self.path = path
        self.file = None
        self.data = None
        self.events = {}
        self.timelines = {}
        self.instruments = {}
        self.waves = {}
        self.samples = []
        self.headers = []
        self.version = 0
        self.event_instruments = {}
        self.sheets = {}

    def __enter__(self):
        self.file = self.path.open("rb")
        try:
            self.data = mmap.mmap(self.file.fileno(), 0, access=mmap.ACCESS_READ)
            if self.data[:4] != b"RIFF" or self.data[8:12] != b"FEV ":
                raise ValueError("Not a FEV bank")
            self._chunks(12, min(len(self.data), 8 + u32(self.data, 4)))
            for offset, length in self.headers:
                self.samples.append(self._fsb(offset, length))
            return self
        except BaseException:
            self.__exit__(None, None, None)
            raise

    def __exit__(self, *_):
        if self.data is not None:
            self.data.close()
        self.file.close()

    def _chunks(self, at, end):
        d = self.data
        owner = None
        while at + 8 <= end:
            kind, length = struct.unpack_from("<4sI", d, at)
            start = at + 8
            stop = start + length
            if stop > end:
                raise ValueError("Truncated FEV chunk")
            if kind == b"FMT ":
                self.version = u32(d, start)
            elif kind == b"LIST":
                self._chunks(start + 4, stop)
            elif kind == b"SNDH":
                entries, _ = integer_array(d[start:stop], 0)
                self.headers = [struct.unpack("<II", entry) for entry in entries]
            elif kind in (b"EVTB", b"TLNB", b"PMLB", b"WAV ", b"WAIB", b"EVIB", b"MUIB", b"SPIB", b"PLST"):
                body = d[start:stop]
                guid = body[:16]
                if kind == b"EVTB" and len(body) >= 48:
                    self.events[guid] = body[32:48]
                    if self.version >= 131 and len(body) >= 95:
                        sheets, i = integer_array(body, 93)
                        for string_values in (False, True):
                            count, i = size_value(body, i)
                            for _ in range(count):
                                length, i = size_value(body, i)
                                i += length
                                if string_values:
                                    length, i = size_value(body, i)
                                    i += length
                                else:
                                    i += 4
                        _, i = integer_array(body, i + 16)
                        _, i = integer_array(body, i)
                        triggered, _ = integer_array(body, i)
                        self.event_instruments[guid] = triggered + sheets
                elif kind == b"TLNB":
                    boxes, i = integer_array(body, 16)
                    locked, _ = integer_array(body, i)
                    self.timelines[guid] = [(b[:16], u32(b, 16) / 48_000) for b in boxes + locked if len(b) == 24]
                elif kind == b"PMLB" and len(body) >= 34:
                    self.sheets[guid], _ = integer_array(body, 32)
                elif kind == b"WAV " and len(body) >= 26:
                    self.waves[guid] = struct.unpack_from("<ii", body, 18)
                elif kind == b"WAIB" and len(body) >= 32:
                    self.instruments[guid] = ("wave", body[16:32])
                elif kind == b"EVIB" and len(body) >= 32:
                    self.instruments[guid] = ("event", body[16:32])
                elif kind in (b"MUIB", b"SPIB"):
                    owner = guid
                elif kind == b"PLST" and owner and len(body) >= 10:
                    entries, _ = integer_array(body, 8)
                    self.instruments[owner] = ("choices", [entry[:16] for entry in entries])
            at = stop + length % 2

    def _fsb(self, base, size):
        d = self.data
        if base + size > len(d) or d[base:base + 4] != b"FSB5":
            raise ValueError("Missing FSB5 sample bank")
        version, count, headers, names, payload, codec = struct.unpack_from("<6I", d, base + 4)
        if codec != 15:
            return []
        at = base + (60 if version else 64)
        strings = at + headers
        start = strings + names
        if count > headers // 8 or start + payload > base + size or count * 4 > names:
            raise ValueError("Invalid FSB5 header")
        samples = []
        for index in range(count):
            header, = struct.unpack_from("<Q", d, at)
            at += 8
            flag = header & 1
            rate_id = header >> 1 & 15
            rate = RATES[rate_id] if rate_id < len(RATES) else 0
            channels = (header >> 5 & 3) + 1
            position = start + ((header >> 7 & 0x7ffffff) * 32)
            frames = header >> 34
            crc = 0
            while flag:
                meta = u32(d, at)
                flag = meta & 1
                length, kind = (meta >> 1) & 0xffffff, meta >> 25
                at += 4
                if at + length > strings:
                    raise ValueError("Truncated FSB5 metadata")
                if kind == 11 and length >= 4:
                    crc = u32(d, at)
                elif kind == 2 and length == 4:
                    rate = u32(d, at)
                elif kind == 1 and length == 1:
                    channels = d[at]
                at += length
            name = strings + u32(d, strings + index * 4)
            if name >= start or position > start + payload:
                raise ValueError("FSB5 sample outside bank")
            terminator = d.find(b"\0", name, start)
            if terminator < 0:
                raise ValueError("Unterminated FSB5 sample name")
            if samples:
                samples[-1].end = position
            samples.append(Sample(d[name:terminator].decode("utf-8", "replace"), position, start + payload, channels, rate, frames, crc))
        return samples

    def sample_for(self, ref):
        bank, index = ref
        return self.samples[bank][index] if 0 <= bank < len(self.samples) and 0 <= index < len(self.samples[bank]) else None

    def clips(self, guid):
        """Keep simultaneous instruments apart; a multi-instrument selects one take."""
        found = {}
        def visit(identifier, time, seen, lane):
            if identifier in seen or len(seen) > 16 or len(found) >= 512:
                return
            seen = seen | {identifier}
            if identifier in self.events:
                for index, (inst, offset) in enumerate(self.timelines.get(self.events[identifier], [])):
                    visit(inst, time + offset, seen, lane + (index,))
                for index, inst in enumerate(self.event_instruments.get(identifier, [])):
                    visit(inst, time, seen, lane + (-1, index))
            elif identifier in self.sheets:
                for inst in self.sheets[identifier]:
                    visit(inst, time, seen, lane)
            elif identifier in self.instruments:
                kind, ref = self.instruments[identifier]
                if kind == "wave" and ref in self.waves:
                    found.setdefault((time, lane), []).append(self.waves[ref])
                elif kind == "choices":
                    for child in ref:
                        visit(child, time, seen, lane)
                elif kind == "event":
                    visit(ref, time, seen, lane)
        visit(UUID(guid).bytes_le, 0, set(), ())
        return [(time, refs) for (time, _), refs in sorted(found.items())]


def ogg_crc(data):
    reflected = zlib.crc32(data.translate(REVERSE), 0xffffffff) ^ 0xffffffff
    return int.from_bytes(reflected.to_bytes(4, "little").translate(REVERSE), "big")


def modes(header):
    # The mode list terminates a Vorbis setup packet. Each mode has the
    # long-block bit, zero window/transform types, and an 8-bit mapping.
    bits = int.from_bytes(header[-340:], "little")
    end = bits.bit_length() - 1
    for n in range(min(64, (end - 6) // 41), 0, -1):
        packed = bits >> (end - n * 41 - 6)
        if packed & 63 == n - 1 and all((packed >> (7 + i * 41)) & 0xffffffff == 0 for i in range(n)):
            return [bool((packed >> (6 + i * 41)) & 1) for i in range(n)]
    raise ValueError("Unsupported Vorbis setup mode list")


def remux(data, sample):
    setup = SETUPS.get(sample.crc)
    if not isinstance(setup, bytes) or not sample.rate:
        return None
    flags = modes(setup)
    mask = (1 << (len(flags) - 1).bit_length()) - 1
    # FSB5 uses 256/2048 transform blocks for the desktop Vorbis stream.
    identification = b"\x01vorbis" + struct.pack("<IBIiiiBB", 0, sample.channels, sample.rate, 0, 0, 0, 0xb8, 1)
    comment = b"\x03vorbis" + struct.pack("<I", 3) + b"DGE" + struct.pack("<IB", 0, 1)
    output = bytearray()
    serial, page_no = 1, 0
    def page(payload, laces, granule, page_flags):
        nonlocal page_no
        body = bytearray(struct.pack("<4sBBqIIIB", b"OggS", 0, page_flags, granule, serial, page_no, 0, len(laces)))
        body.extend(laces)
        body.extend(payload)
        struct.pack_into("<I", body, 22, ogg_crc(body))
        output.extend(body)
        page_no += 1

    def lace(n):
        return [255] * (n // 255) + [n % 255]
    page(identification, lace(len(identification)), 0, 2)
    page(comment + setup, lace(len(comment)) + lace(len(setup)), 0, 0)
    parts, segments = bytearray(), []
    at, granule, prev = sample.start, 0, 0
    while at + 2 <= sample.end:
        size = u16(data, at)
        at += 2
        if size in (0, 0xffff):
            break
        if at + size > sample.end or size > 65024:
            return None
        packet = data[at:at + size]
        at += size
        mode = (packet[0] >> 1) & mask
        if packet[0] & 1 or mode >= len(flags):
            return None
        packet_laces = lace(size)
        if len(segments) + len(packet_laces) > 255 or len(parts) >= 8192:
            page(parts, segments, granule, 0)
            parts, segments = bytearray(), []
        block = 2048 if flags[mode] else 256
        if prev:
            granule += (block + prev) // 4
        prev = block
        parts.extend(packet)
        segments.extend(packet_laces)
    if not segments:
        return None
    page(parts, segments, min(sample.frames, granule), 4)
    return bytes(output)
