package speech

// accented and plain pair each Latin letter that is a plain letter with
// accents (its canonical decomposition, Unicode's NFD, starts with that
// letter) with the plain letter, over Latin-1 and Latin Extended-A: the
// letters of the languages LingoReel has speech models for, and their
// neighbours. A letter with no such decomposition (ø, æ, ß, đ, ł) is not in
// it: it is a letter of its own.
const (
	accented = "ÀÁÂÃÄÅÇÈÉÊËÌÍÎÏÑÒÓÔÕÖÙÚÛÜÝàáâãäåçèéêëìíîïñòóôõöùúûüýÿ" +
		"ĀāĂăĄąĆćĈĉĊċČčĎďĒēĔĕĖėĘęĚěĜĝĞğĠġĢģĤĥĨĩĪīĬĭĮįİĴĵĶķĹĺĻļĽľŃńŅņŇňŌōŎŏŐőŔŕŖŗŘřŚśŜŝŞşŠšŢţŤťŨũŪūŬŭŮůŰűŲųŴŵŶŷŸŹźŻżŽž"
	plain = "AAAAAACEEEEIIIINOOOOOUUUUYaaaaaaceeeeiiiinooooouuuuyy" +
		"AaAaAaCcCcCcCcDdEeEeEeEeEeGgGgGgGgHhIiIiIiIiIJjKkLlLlLlNnNnNnOoOoOoRrRrRrSsSsSsSsTtTtUuUuUuUuUuUuWwYyYZzZzZz"
)

// plainLetters maps each accented letter to its plain one.
var plainLetters = func() map[rune]rune {
	from, to := []rune(accented), []rune(plain)
	letters := make(map[rune]rune, len(from))
	for i, r := range from {
		letters[r] = to[i]
	}
	return letters
}()

// plainLetter is a Latin letter without its accents ("é" as "e"); false for
// a letter that has none.
func plainLetter(r rune) (rune, bool) {
	p, ok := plainLetters[r]
	return p, ok
}
