// Package matroska reads the parts of Matroska/WebM files that the LingoReel needs: segment information, tracks and the frames of subtitle tracks.
// Every file is treated as untrusted input and every allocation is bounded.
package matroska

// EBML element IDs, with their length marker bits, as defined by the Matroska
// specification (RFC 9559).
const (
	idEBML       = 0x1A45DFA3
	idDocType    = 0x4282
	idSegment    = 0x18538067
	idSeekHead   = 0x114D9B74
	idSeek       = 0x4DBB
	idSeekID     = 0x53AB
	idSeekPos    = 0x53AC
	idInfo       = 0x1549A966
	idTimescale  = 0x2AD7B1
	idDuration   = 0x4489
	idTracks     = 0x1654AE6B
	idTrackEntry = 0xAE
	idCluster    = 0x1F43B675
	idCues       = 0x1C53BB6B
	idTags       = 0x1254C367
	idChapters   = 0x1043A770
	idAttachment = 0x1941A469

	idTrackNumber         = 0xD7
	idTrackUID            = 0x73C5
	idTrackType           = 0x83
	idFlagDefault         = 0x88
	idFlagForced          = 0x55AA
	idFlagHearingImpaired = 0x55AB
	idDefaultDuration     = 0x23E383
	idName                = 0x536E
	idLanguage            = 0x22B59C
	idLanguageBCP47       = 0x22B59D
	idCodecID             = 0x86
	idVideo               = 0xE0
	idPixelWidth          = 0xB0
	idPixelHeight         = 0xBA

	idContentEncodings     = 0x6D80
	idContentEncoding      = 0x6240
	idContentEncodingOrder = 0x5031
	idContentEncodingScope = 0x5032
	idContentEncodingType  = 0x5033
	idContentCompression   = 0x5034
	idContentCompAlgo      = 0x4254
	idContentCompSettings  = 0x4255

	idClusterTimestamp = 0xE7
	idSimpleBlock      = 0xA3
	idBlockGroup       = 0xA0
	idBlock            = 0xA1
	idBlockDuration    = 0x9B

	idCuePoint            = 0xBB
	idCueTrackPositions   = 0xB7
	idCueTrack            = 0xF7
	idCueClusterPosition  = 0xF1
	idCueRelativePosition = 0xF0
)

// topLevelIDs end an unknown-size cluster: once one of them appears, the
// cluster is over.
var topLevelIDs = map[uint32]bool{
	idSeekHead: true, idInfo: true, idTracks: true, idCluster: true,
	idCues: true, idTags: true, idChapters: true, idAttachment: true,
}

// Matroska track types.
const (
	trackTypeVideo    = 1
	trackTypeAudio    = 2
	trackTypeSubtitle = 0x11
)
